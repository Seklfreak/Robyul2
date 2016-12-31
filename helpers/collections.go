package helpers

import (
    "errors"
    "reflect"
    "math/rand"
)

// interfaceToSlice converts an interface to []interface{}
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

// SliceRemoveOrderedElement removes an element from a slice but keeps the order (slow)
func SliceRemoveOrderedElement(input interface{}, idx int) interface{} {
    array := interfaceToSlice(input)
    return append(array[:idx], array[idx + 1:]...)
}

// SliceRemoveElement removes an element from a slice but ignores the element order (fast)
func SliceRemoveElement(input interface{}, idx int) interface{} {
    array := interfaceToSlice(input)
    array[len(array) - 1], array[idx] = array[idx], array[len(array) - 1]
    return array[:len(array) - 1]
}

// SliceRandom returns a random element from a slice
func SliceRandom(input interface{}) interface{} {
    array := interfaceToSlice(input)
    return array[rand.Intn(len(array) - 1)]
}
