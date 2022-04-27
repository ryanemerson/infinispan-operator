// Copyright 2013 Dario Castañé. All rights reserved.
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Based on src/pkg/reflect/deepequal.go from official
// golang's stdlib.

// Modified version of the https://github.com/imdario/mergo library that updates the behaviour of WithSliceDeepCopy
// to better suit our needs

package mergo

import (
	"errors"
	"fmt"
	"reflect"
)

// Errors reported by Mergo when it finds invalid arguments.
var (
	ErrNilArguments                = errors.New("src and dst must not be nil")
	ErrDifferentArgumentsTypes     = errors.New("src and dst must be of same type")
	ErrNotSupported                = errors.New("only structs and maps are supported")
	ErrExpectedMapAsDestination    = errors.New("dst was expected to be a map")
	ErrExpectedStructAsDestination = errors.New("dst was expected to be a struct")
	ErrNonPointerAgument           = errors.New("dst must be a pointer")
)

// During deepMerge, must keep track of checks that are
// in progress.  The comparison algorithm assumes that all
// checks in progress are true when it reencounters them.
// Visited are stored in a map indexed by 17 * a1 + a2;
type visit struct {
	ptr  uintptr
	typ  reflect.Type
	next *visit
}

// From src/pkg/encoding/json/encode.go.
func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		if v.IsNil() {
			return true
		}
		return isEmptyValue(v.Elem())
	case reflect.Func:
		return v.IsNil()
	case reflect.Invalid:
		return true
	}
	return false
}

func resolveValues(dst, src interface{}) (vDst, vSrc reflect.Value, err error) {
	if dst == nil || src == nil {
		err = ErrNilArguments
		return
	}
	vDst = reflect.ValueOf(dst).Elem()
	if vDst.Kind() != reflect.Struct && vDst.Kind() != reflect.Map {
		fmt.Println(vDst.Kind())
		err = ErrNotSupported
		return
	}
	vSrc = reflect.ValueOf(src)
	// We check if vSrc is a pointer to dereference it.
	if vSrc.Kind() == reflect.Ptr {
		vSrc = vSrc.Elem()
	}
	return
}

func hasMergeableFields(dst reflect.Value) (exported bool) {
	for i, n := 0, dst.NumField(); i < n; i++ {
		field := dst.Type().Field(i)
		if field.Anonymous && dst.Field(i).Kind() == reflect.Struct {
			exported = exported || hasMergeableFields(dst.Field(i))
		} else if isExportedComponent(&field) {
			exported = exported || len(field.PkgPath) == 0
		}
	}
	return
}

func isExportedComponent(field *reflect.StructField) bool {
	pkgPath := field.PkgPath
	if len(pkgPath) > 0 {
		return false
	}
	c := field.Name[0]
	if 'a' <= c && c <= 'z' || c == '_' {
		return false
	}
	return true
}

type Config struct {
	Overwrite                    bool
	AppendSlice                  bool
	TypeCheck                    bool
	Transformers                 Transformers
	overwriteWithEmptyValue      bool
	overwriteSliceWithEmptyValue bool
	sliceDeepCopy                bool
	debug                        bool
}

type Transformers interface {
	Transformer(dst, src reflect.Value) (bool, error)
}

// Traverses recursively both values, assigning src's fields values to dst.
// The map argument tracks comparisons that have already been seen, which allows
// short circuiting on recursive types.
func deepMerge(dst, src reflect.Value, visited map[uintptr]*visit, depth int, config *Config) (err error) {
	overwrite := config.Overwrite
	typeCheck := config.TypeCheck
	overwriteWithEmptySrc := config.overwriteWithEmptyValue
	overwriteSliceWithEmptySrc := config.overwriteSliceWithEmptyValue
	sliceDeepCopy := config.sliceDeepCopy

	//if src.IsNil() {
	//	fmt.Printf("SrcElement=Nil\n")
	//} else {
	//	fmt.Printf("SrcElement=%s | SrcInterface=%s\n", src.Kind(), reflect.TypeOf(src.Interface()).Kind())
	//}

	if !src.IsValid() {
		return
	}

	if dst.CanAddr() {
		addr := dst.UnsafeAddr()
		h := 17 * addr
		seen := visited[h]
		typ := dst.Type()
		for p := seen; p != nil; p = p.next {
			if p.ptr == addr && p.typ == typ {
				return nil
			}
		}
		// Remember, remember...
		visited[h] = &visit{addr, typ, seen}
	}

	//if config.Transformers != nil && !isEmptyValue(dst) {
	//	var transformed bool
	//	transformed, err = config.Transformers.Transformer(dst, src)
	//	if transformed {
	//		return
	//	}
	//}

	switch dst.Kind() {
	case reflect.Struct:
		if hasMergeableFields(dst) {
			for i, n := 0, dst.NumField(); i < n; i++ {
				if err = deepMerge(dst.Field(i), src.Field(i), visited, depth+1, config); err != nil {
					return
				}
			}
		} else {
			if dst.CanSet() && (isReflectNil(dst) || overwrite) && (!isEmptyValue(src) || overwriteWithEmptySrc) {
				dst.Set(src)
			}
		}
	case reflect.Map:
		if dst.IsNil() && !src.IsNil() {
			if dst.CanSet() {
				dst.Set(reflect.MakeMap(dst.Type()))
			} else {
				dst = src
				return
			}
		}

		if src.Kind() != reflect.Map {
			if overwrite {
				dst.Set(src)
			}
			return
		}

		for _, key := range src.MapKeys() {
			srcElement := src.MapIndex(key)
			//if srcElement.IsNil() {
			//	fmt.Printf("Key=%s | SrcElement=Nil\n", key)
			//} else {
			//	fmt.Printf("Key=%s | SrcElement=%s | SrcInterface=%s\n", key, srcElement.Kind(), reflect.TypeOf(srcElement.Interface()).Kind())
			//}
			if !srcElement.IsValid() {
				continue
			}
			dstElement := dst.MapIndex(key)
			switch srcElement.Kind() {
			case reflect.Chan, reflect.Func, reflect.Map, reflect.Interface, reflect.Slice:
				if srcElement.IsNil() {
					if overwrite {
						dst.SetMapIndex(key, srcElement)
					}
					continue
				}
				fallthrough
			default:
				if !srcElement.CanInterface() {
					continue
				}
				switch reflect.TypeOf(srcElement.Interface()).Kind() {
				case reflect.Struct:
					fallthrough
				case reflect.Ptr:
					fallthrough
				case reflect.Map:
					srcMapElm := srcElement
					dstMapElm := dstElement
					if srcMapElm.CanInterface() {
						srcMapElm = reflect.ValueOf(srcMapElm.Interface())
						if dstMapElm.IsValid() {
							dstMapElm = reflect.ValueOf(dstMapElm.Interface())
						}
					}
					if err = deepMerge(dstMapElm, srcMapElm, visited, depth+1, config); err != nil {
						return
					}
				case reflect.Slice:
					srcSlice := reflect.ValueOf(srcElement.Interface())

					var dstSlice reflect.Value
					if !dstElement.IsValid() || dstElement.IsNil() {
						dstSlice = reflect.MakeSlice(srcSlice.Type(), 0, srcSlice.Len())
					} else {
						dstSlice = reflect.ValueOf(dstElement.Interface())
					}

					if (!isEmptyValue(src) || overwriteWithEmptySrc || overwriteSliceWithEmptySrc) && (overwrite || isEmptyValue(dst)) && !config.AppendSlice && !sliceDeepCopy {
						if typeCheck && srcSlice.Type() != dstSlice.Type() {
							return fmt.Errorf("cannot override two slices with different type (%s, %s)", srcSlice.Type(), dstSlice.Type())
						}
						dstSlice = srcSlice
					} else if config.AppendSlice {
						if srcSlice.Type() != dstSlice.Type() {
							return fmt.Errorf("cannot append two slices with different type (%s, %s)", srcSlice.Type(), dstSlice.Type())
						}
						dstSlice = reflect.AppendSlice(dstSlice, srcSlice)
					} else if sliceDeepCopy {
						// Modified behaviour of the original Mergo project
						// Keep the merge behaviour when src and dst slice length is the same, or src < dst
						// Always merge the full src array if it's greater than the dst
						srcLen := srcSlice.Len()
						dstLen := dstSlice.Len()

						if srcLen == dstLen {
							for i := 0; i < srcLen && i < dstLen; i++ {
								srcElement := srcSlice.Index(i)
								dstElement := dstSlice.Index(i)

								if srcElement.CanInterface() {
									srcElement = reflect.ValueOf(srcElement.Interface())
								}
								if dstElement.CanInterface() {
									dstElement = reflect.ValueOf(dstElement.Interface())
								}

								srcVal, e := srcElement.Interface().(string)
								if e {
									fmt.Printf("Slice:=%s|err=%v\n", srcVal, e)
									dstVal, e := dstElement.Interface().(string)
									fmt.Printf("Slice:=%s|err=%v\n", dstVal, e)
								}

								switch reflect.TypeOf(srcElement.Interface()).Kind() {
								case reflect.String:
									// The dst element is not addressable so overwriting of individual elements in the
									// slice won't work, therefore we always use the src slice in full
									dstSlice = srcSlice
									break
								}

								if err = deepMerge(dstElement, srcElement, visited, depth+1, config); err != nil {
									return
								}
							}
						} else {
							dstSlice = srcSlice
						}
					}
					dst.SetMapIndex(key, dstSlice)
				}
			}
			if dstElement.IsValid() && !isEmptyValue(dstElement) && (reflect.TypeOf(srcElement.Interface()).Kind() == reflect.Map || reflect.TypeOf(srcElement.Interface()).Kind() == reflect.Slice) {
				continue
			}

			if srcElement.IsValid() && ((srcElement.Kind() != reflect.Ptr && overwrite) || !dstElement.IsValid() || isEmptyValue(dstElement)) {
				if dst.IsNil() {
					dst.Set(reflect.MakeMap(dst.Type()))
				}
				dst.SetMapIndex(key, srcElement)
			}
		}
	case reflect.Slice:
		if !dst.CanSet() {
			break
		}
		if (!isEmptyValue(src) || overwriteWithEmptySrc || overwriteSliceWithEmptySrc) && (overwrite || isEmptyValue(dst)) && !config.AppendSlice && !sliceDeepCopy {
			dst.Set(src)
		} else if config.AppendSlice {
			if src.Type() != dst.Type() {
				return fmt.Errorf("cannot append two slice with different type (%s, %s)", src.Type(), dst.Type())
			}
			dst.Set(reflect.AppendSlice(dst, src))
		} else if sliceDeepCopy {
			// Modified behaviour of the original Mergo project
			// Keep the merge behaviour when src and dst slice length is the same, or src < dst
			// Always merge the full src array if it's greater than the dst
			srcLen := src.Len()
			dstLen := dst.Len()

			if srcLen == dstLen {
				for i := 0; i < srcLen && i < dstLen; i++ {
					srcElement := src.Index(i)
					dstElement := dst.Index(i)

					if srcElement.CanInterface() {
						srcElement = reflect.ValueOf(srcElement.Interface())
					}
					if dstElement.CanInterface() {
						dstElement = reflect.ValueOf(dstElement.Interface())
					}

					fmt.Println(srcElement.Kind())
					srcVal, e := srcElement.Interface().(string)
					if e {
						fmt.Printf("NativeSlice:=%s|err=%v\n", srcVal, e)
						dstVal, e := dstElement.Interface().(string)
						fmt.Printf("NativeSlice:=%s|err=%v\n", dstVal, e)
					}

					switch reflect.TypeOf(srcElement.Interface()).Kind() {
					case reflect.String:
						// The dst element is not addressable so overwriting of individual elements in the
						// slice won't work, therefore we always use the src slice in full
						dst.Set(src)
						break
					default:
						fmt.Println(reflect.TypeOf(srcElement.Interface()).Kind())
					}

					if err = deepMerge(dstElement, srcElement, visited, depth+1, config); err != nil {
						return
					}
				}
			} else {
				dst.Set(src)
			}
		}
	case reflect.Ptr:
		fallthrough
	case reflect.Interface:
		if isReflectNil(src) {
			if overwriteWithEmptySrc && dst.CanSet() && src.Type().AssignableTo(dst.Type()) {
				dst.Set(src)
			}
			break
		}

		if src.Kind() != reflect.Interface {
			if dst.IsNil() || (src.Kind() != reflect.Ptr && overwrite) {
				if dst.CanSet() && (overwrite || isEmptyValue(dst)) {
					dst.Set(src)
				}
			} else if src.Kind() == reflect.Ptr {
				if err = deepMerge(dst.Elem(), src.Elem(), visited, depth+1, config); err != nil {
					return
				}
			} else if dst.Elem().Type() == src.Type() {
				if err = deepMerge(dst.Elem(), src, visited, depth+1, config); err != nil {
					return
				}
			} else {
				return ErrDifferentArgumentsTypes
			}
			break
		}

		if dst.IsNil() || overwrite {
			if dst.CanSet() && (overwrite || isEmptyValue(dst)) {
				dst.Set(src)
			}
			break
		}

		if dst.Elem().Kind() == src.Elem().Kind() {
			if err = deepMerge(dst.Elem(), src.Elem(), visited, depth+1, config); err != nil {
				return
			}
			break
		}
	default:
		mustSet := (isEmptyValue(dst) || overwrite) && (!isEmptyValue(src) || overwriteWithEmptySrc)
		if mustSet {
			if dst.CanSet() {
				dst.Set(src)
			} else {
				dst = src
			}
		}
	}

	return
}

// Merge will fill any empty for value type attributes on the dst struct using corresponding
// src attributes if they themselves are not empty. dst and src must be valid same-type structs
// and dst must be a pointer to struct.
// It won't merge unexported (private) fields and will do recursively any exported field.
func Merge(dst, src interface{}, opts ...func(*Config)) error {
	return merge(dst, src, opts...)
}

// MergeWithOverwrite will do the same as Merge except that non-empty dst attributes will be overridden by
// non-empty src attribute values.
// Deprecated: use Merge(…) with WithOverride
func MergeWithOverwrite(dst, src interface{}, opts ...func(*Config)) error {
	return merge(dst, src, append(opts, WithOverride)...)
}

// WithTransformers adds transformers to merge, allowing to customize the merging of some types.
func WithTransformers(transformers Transformers) func(*Config) {
	return func(config *Config) {
		config.Transformers = transformers
	}
}

// WithOverride will make merge override non-empty dst attributes with non-empty src attributes values.
func WithOverride(config *Config) {
	config.Overwrite = true
}

// WithOverwriteWithEmptyValue will make merge override non empty dst attributes with empty src attributes values.
func WithOverwriteWithEmptyValue(config *Config) {
	config.Overwrite = true
	config.overwriteWithEmptyValue = true
}

// WithOverrideEmptySlice will make merge override empty dst slice with empty src slice.
func WithOverrideEmptySlice(config *Config) {
	config.overwriteSliceWithEmptyValue = true
}

// WithAppendSlice will make merge append slices instead of overwriting it.
func WithAppendSlice(config *Config) {
	config.AppendSlice = true
}

// WithTypeCheck will make merge check types while overwriting it (must be used with WithOverride).
func WithTypeCheck(config *Config) {
	config.TypeCheck = true
}

// WithSliceDeepCopy will merge slice element one by one with Overwrite flag.
func WithSliceDeepCopy(config *Config) {
	config.sliceDeepCopy = true
	config.Overwrite = true
}

func merge(dst, src interface{}, opts ...func(*Config)) error {
	if dst != nil && reflect.ValueOf(dst).Kind() != reflect.Ptr {
		return ErrNonPointerAgument
	}
	var (
		vDst, vSrc reflect.Value
		err        error
	)

	config := &Config{}

	for _, opt := range opts {
		opt(config)
	}

	if vDst, vSrc, err = resolveValues(dst, src); err != nil {
		return err
	}
	if vDst.Type() != vSrc.Type() {
		return ErrDifferentArgumentsTypes
	}
	return deepMerge(vDst, vSrc, make(map[uintptr]*visit), 0, config)
}

// IsReflectNil is the reflect value provided nil
func isReflectNil(v reflect.Value) bool {
	k := v.Kind()
	switch k {
	case reflect.Interface, reflect.Slice, reflect.Chan, reflect.Func, reflect.Map, reflect.Ptr:
		// Both interface and slice are nil if first word is 0.
		// Both are always bigger than a word; assume flagIndir.
		return v.IsNil()
	default:
		return false
	}
}
