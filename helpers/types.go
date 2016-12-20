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
