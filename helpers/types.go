package helpers

import "reflect"

// Resolves the type of $v as a string
func Typeof(v interface{}) string {
    t := reflect.TypeOf(v)

    if t.Kind() == reflect.Ptr {
        return "*" + t.Elem().Name()
    } else {
        return t.Name()
    }
}
