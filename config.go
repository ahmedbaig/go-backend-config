package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
)

const (
	DefaultDir = "/app/var/secrets"
	DirEnv     = "CONFIG_DIR"
)

type Source string

const (
	SourceEnv     Source = "env"
	SourceFile    Source = "file"
	SourceDefault Source = "default"
)

type Validator interface {
	Validate() error
}

type options struct {
	dir      string
	envKey   string
	strict   bool
	validate *validator.Validate
	lookup   func(string) (string, bool)
}

type Option func(*options)

func WithDir(dir string) Option {
	return func(o *options) { o.dir = dir }
}

func WithEnvKey(key string) Option {
	return func(o *options) { o.envKey = key }
}

func WithStrict() Option {
	return func(o *options) { o.strict = true }
}

func WithValidator(v *validator.Validate) Option {
	return func(o *options) { o.validate = v }
}

func WithLookup(fn func(string) (string, bool)) Option {
	return func(o *options) { o.lookup = fn }
}

var nonWord = regexp.MustCompile(`\W`)

func EnvKey(name string) string {
	return strings.ToUpper(nonWord.ReplaceAllString(name, "_")) + "_SECRET"
}

func Dir() string {
	if d := os.Getenv(DirEnv); d != "" {
		return d
	}
	return DefaultDir
}

type Result[T any] struct {
	Value  T
	Source Source
	Path   string
}

func Load[T any](name string, defaults T, opts ...Option) (T, error) {
	r, err := LoadWithSource(name, defaults, opts...)
	return r.Value, err
}

func MustLoad[T any](name string, defaults T, opts ...Option) T {
	v, err := Load(name, defaults, opts...)
	if err != nil {
		panic(err)
	}
	return v
}

var sharedValidator = validator.New(validator.WithRequiredStructEnabled())

func LoadWithSource[T any](name string, defaults T, opts ...Option) (Result[T], error) {
	o := options{dir: Dir(), envKey: EnvKey(name), validate: sharedValidator, lookup: os.LookupEnv}
	for _, opt := range opts {
		opt(&o)
	}

	res := Result[T]{Value: defaults, Source: SourceDefault}

	raw, src, path, err := read(name, o)
	if err != nil {
		return res, err
	}
	if raw != nil {
		if err := decode(raw, &res.Value, o.strict); err != nil {
			return res, fmt.Errorf("config %q from %s: %w", name, src, err)
		}
		res.Source, res.Path = src, path
	}

	if err := check(&res.Value, o.validate); err != nil {
		return res, fmt.Errorf("config %q invalid: %w", name, err)
	}
	return res, nil
}

func read(name string, o options) ([]byte, Source, string, error) {
	if v, ok := o.lookup(o.envKey); ok && strings.TrimSpace(v) != "" {
		return []byte(v), SourceEnv, o.envKey, nil
	}
	path := filepath.Join(o.dir, name+".json")
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, SourceDefault, "", nil
	}
	if err != nil {
		return nil, SourceFile, path, fmt.Errorf("config %q: %w", name, err)
	}
	return b, SourceFile, path, nil
}

func decode(raw []byte, into any, strict bool) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	if strict {
		dec.DisallowUnknownFields()
	}
	return dec.Decode(into)
}

func check(ptr any, val *validator.Validate) error {
	rv := reflect.ValueOf(ptr).Elem()
	target := rv
	for target.Kind() == reflect.Pointer && !target.IsNil() {
		target = target.Elem()
	}
	if val != nil && target.Kind() == reflect.Struct {
		if err := val.Struct(target.Interface()); err != nil {
			return err
		}
	}
	if c, ok := ptr.(Validator); ok {
		return c.Validate()
	}
	if c, ok := rv.Interface().(Validator); ok {
		return c.Validate()
	}
	return nil
}
