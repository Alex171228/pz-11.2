package cache

import "fmt"

func TaskByIDKey(id string) string {
	return fmt.Sprintf("tasks:task:%s", id)
}
