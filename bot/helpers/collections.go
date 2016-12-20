package helpers

import (
    "reflect"
    "errors"
)

func interfaceToSlice(input interface{}) []interface{} {
    r := reflect.ValueOf(input)

    if r.Kind() != reflect.Slice {
        panic(errors.New("Passed arg is not a slice"))
    }

    c := r.Len()
    s := make([]interface{}, c)

    for i := 0; i < c; i++ {
        s[i] = r.Index(i).Interface()
    }

    return s
}

func SliceRemoveOrderedElement(input interface{}, idx int) []interface{} {
    array := interfaceToSlice(input)
    return append(array[:idx], array[idx + 1:]...)
}

func SliceRemoveElement(input interface{}, idx int) []interface{} {
    array := interfaceToSlice(input)
    array[len(array) - 1], array[idx] = array[idx], array[len(array) - 1]
    return array[:len(array) - 1]
}
