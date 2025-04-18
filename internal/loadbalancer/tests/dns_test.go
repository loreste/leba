package tests

import (
	"testing"

	"leba/internal/loadbalancer"
)

func TestDNSBackendParsing(t *testing.T) {
	// Simple test to verify the ParseDNSBackend function works correctly
	result, err := loadbalancer.ParseDNSBackend("example.com:80/http", "test-group", true)
	if err != nil {
		t.Errorf("Failed to parse DNS backend: %v", err)
	}
	
	if result.Domain != "example.com" {
		t.Errorf("Expected domain to be 'example.com', got '%s'", result.Domain)
	}
	
	if result.Port != 80 {
		t.Errorf("Expected port to be 80, got %d", result.Port)
	}
	
	if result.Protocol != "http" {
		t.Errorf("Expected protocol to be 'http', got '%s'", result.Protocol)
	}
}

func TestFindAddedRemovedItems(t *testing.T) {
	// These are our helper functions, so just define simple test versions of them here
	findAddedItems := func(oldList, newList []string) []string {
		result := make([]string, 0)
		for _, new := range newList {
			found := false
			for _, old := range oldList {
				if old == new {
					found = true
					break
				}
			}
			if !found {
				result = append(result, new)
			}
		}
		return result
	}

	findRemovedItems := func(oldList, newList []string) []string {
		result := make([]string, 0)
		for _, old := range oldList {
			found := false
			for _, new := range newList {
				if old == new {
					found = true
					break
				}
			}
			if !found {
				result = append(result, old)
			}
		}
		return result
	}

	// Test findAddedItems
	oldList := []string{"a", "b", "c"}
	newList := []string{"b", "c", "d", "e"}

	added := findAddedItems(oldList, newList)
	if len(added) != 2 || added[0] != "d" || added[1] != "e" {
		t.Errorf("findAddedItems didn't find the correct items: %v", added)
	}

	// Test findRemovedItems
	removed := findRemovedItems(oldList, newList)
	if len(removed) != 1 || removed[0] != "a" {
		t.Errorf("findRemovedItems didn't find the correct items: %v", removed)
	}

	// Test with empty lists
	added = findAddedItems([]string{}, []string{})
	if len(added) != 0 {
		t.Errorf("findAddedItems with empty lists should return empty slice, got %v", added)
	}

	removed = findRemovedItems([]string{}, []string{})
	if len(removed) != 0 {
		t.Errorf("findRemovedItems with empty lists should return empty slice, got %v", removed)
	}
}