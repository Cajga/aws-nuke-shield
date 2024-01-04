package helpers

import "strings"

/* Define a custom flag type that stores a list of values as a single argument */
type StringListFlag []string

// Interface method for the custom StringListFlag
func (slf *StringListFlag) String() string {
    return strings.Join(*slf, ", ")
}

// Interface method for the custom StringListFlag
func (slf *StringListFlag) Set(value string) error {
    elements := strings.Split(value, ",")
    *slf = append(*slf, elements...)
    return nil
}
/**/

// Return a new slice created from the one provided, with duplicate items removed
func RemoveDuplicates[T comparable](slice []T) []T {
    for i := 0; i < len(slice); i++ {
        for j := len(slice) - 1; j > i; j-- {
            if slice[i] == slice[j] {
                slice = append(slice[:j], slice[j+1:]...)
            }
        }
    }
    return slice
}

// Return the index of the first item in the given slice which exactly matches the given target string
func FindItemExact(arr []string, target string) int {
    for i, item := range arr {
        if item == target {
            return i // Return the index of the item if found
        }
    }
    return -1 // Return -1 if the item is not found
}

// Return the index of the first item in the given slice which contains the given target string
func FindItem(arr []string, target string) int {
    for i, item := range arr {
        if strings.Contains(item, target) {
            return i // Return the index of the item if found
        }
    }
    return -1 // Return -1 if the item is not found
}

// Return a slice of strings containing all items in the given slice which contain the given target
func FindItemAll(arr []string, target string) []string {
    var matches []string
    for _, item := range arr {
        if strings.Contains(item, target) {
            matches = append(matches, item)
        }
    }
    return matches // Return the slice of matches. If none, this will be empty
}
