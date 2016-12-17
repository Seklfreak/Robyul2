package helpers

import "reflect"

// Checks if the slice $s contains $e
func SliceContains(s []interface{}, e interface{}) bool {
    for _, a := range s {
        if reflect.DeepEqual(a, e) {
            return true
        }
    }

    return false
}
