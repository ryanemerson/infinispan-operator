package kubernetes

import (
	"fmt"
	"github.com/infinispan/infinispan-operator/pkg/mergo"
	corev1 "k8s.io/api/core/v1"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TODO can we just do the merge with Mergo and then manually overwrite the problem fields after the merge?
//
func Merge(merged, left, right client.Object) error {
	//leftType := reflect.ValueOf(left).Type()
	//rightType := reflect.ValueOf(right).Type()
	//if leftType != rightType {
	//	return fmt.Errorf("merge objects type mismatch. Left=%s, Right=%s", leftType, rightType)
	//}
	//
	//leftUnstr, err := runtime.DefaultUnstructuredConverter.ToUnstructured(left.DeepCopyObject())
	//if err != nil {
	//	return fmt.Errorf("unable to convert left object to unstructured: %w", err)
	//}
	//
	//rightUnstr, err := runtime.DefaultUnstructuredConverter.ToUnstructured(right.DeepCopyObject())
	//if err != nil {
	//	return fmt.Errorf("unable to convert right object to unstructured: %w", err)
	//}
	//
	//// Merge the right changes into the left object so that the left fields always win
	//if err = mergo.Merge(&leftUnstr, rightUnstr, mergo.WithSliceDeepCopy, mergo.WithTransformers(transformer{})); err != nil {
	//	return fmt.Errorf("unable to merge unstructured objects: %s", err)
	//}
	//// Convert the unstructured content to the merged object
	//return runtime.DefaultUnstructuredConverter.FromUnstructured(leftUnstr, merged)

	return mergo.Merge(merged, right, mergo.WithSliceDeepCopy, mergo.WithTransformers(Transformer{}))
}

type Transformer struct {
}

func (t Transformer) Transformer(dst, src reflect.Value) (bool, error) {
	dstType := dst.Type()
	fmt.Printf("Type=%s", dstType)
	if dstType.AssignableTo(reflect.TypeOf(corev1.Container{})) {
		// TODO update transformer mechanism to allow the dst and src value to be passed so that we can take action if object is of certain type
		fmt.Println(dst.Interface().(corev1.Container))
		fmt.Println(src.Interface().(corev1.Container))

		//dstContainer := dst.Interface().(corev1.Container)
		//srcContainer := src.Interface().(corev1.Container)

		//dst.FieldByName("Args").Set(src.FieldByName("Args"))
	} else if dstType.AssignableTo(reflect.TypeOf([]string{})) {
		fmt.Println("Ver inter")
		fmt.Printf("CanAddr=%v|CanSet=%v|CanInterface=%v\n", dst.CanSet(), dst.CanAddr(), dst.CanInterface())
		dstArray := dst.Interface().([]string)
		//dst.Set(src)
		//dst.Slice(-1, 0)
		//dstArray[0] = "FUCK!!!"
		fmt.Println(dstArray)
		srcArray := src.Interface().([]string)
		fmt.Println(srcArray)

		if src.IsNil() || src.IsZero() {
			dst.Set(src)
		} else {

		}
		//dst.Set(src)
		//return true, nil
	} else if dstType.AssignableTo(reflect.TypeOf([]corev1.Container{})) {
		fmt.Println("Ver interasfasas")
	}
	return false, nil
}

//func (t Transformer) Transformer(dst reflect.Value) func(dst, src reflect.Value) error {
//	fmt.Printf("Transformer Kind:=%s\n", typ.Kind())
//	if typ.Kind() == reflect.String {
//		return func(dst, src reflect.Value) error {
//			fmt.Println(src.Kind())
//			if src.CanInterface() {
//				srcVal, _ := src.Interface().(string)
//				// Always use the src string value if set
//				if srcVal != "" {
//					if dst.CanSet() {
//						dst.Set(src)
//					} else {
//						dst = src
//					}
//				}
//				//fmt.Println(srcVal)
//				dstVal, _ := dst.Interface().(string)
//				fmt.Println(dstVal)
//			} else {
//				mustSet := (isEmptyValue(dst) || true) && (!isEmptyValue(src) || false)
//				if mustSet {
//					if dst.CanSet() {
//						dst.Set(src)
//					} else {
//						dst = src
//					}
//				}
//			}
//			return nil
//		}
//	}
//	return nil
//}

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
