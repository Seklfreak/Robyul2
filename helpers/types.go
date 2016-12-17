package helpers

import "reflect"

func Typeof(v interface{}) string {
    t := reflect.TypeOf(v)

    if t.Kind() == reflect.Ptr {
        return "*" + t.Elem().Name()
    } else {
        return t.Name()
    }
}
