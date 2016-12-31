package helpers

import "reflect"

// Typeof resolves the type of $v as a string
func Typeof(v interface{}) string {
    t := reflect.TypeOf(v)

    if t.Kind() == reflect.Ptr {
        return "*" + t.Elem().Name()
    }

    return t.Name()
}

// MapToSliceOfSlices converts a map to [][]string
func MapToSliceOfSlices(m map[string]string) [][]string {
    res := make([][]string, len(m))

    for idx := range res {
        for key, val := range m {
            res[idx] = append(res[idx], key)
            res[idx] = append(res[idx], val)
        }
    }

    return res
}
