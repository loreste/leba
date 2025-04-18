//go:build !integration
package loadbalancer

import (
	"context"
	"testing"
)

// mockResolver implements the DNSResolver interface for testing
type mockResolver struct {
	addresses map[string][]string
}

// LookupHost implements the DNSResolver interface
func (r *mockResolver) LookupHost(ctx context.Context, host string) ([]string, error) {
	if addrs, ok := r.addresses[host]; ok {
		return addrs, nil
	}
	return []string{}, nil
}

func TestParseDNSBackend(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		groupName string
		isPrimary bool
		wantErr   bool
		domain    string
		protocol  string
		port      int
	}{
		{
			name:      "valid input",
			input:     "example.com:80/http",
			groupName: "test-group",
			isPrimary: true,
			wantErr:   false,
			domain:    "example.com",
			protocol:  "http",
			port:      80,
		},
		{
			name:      "invalid format",
			input:     "example.com:80",
			groupName: "",
			isPrimary: false,
			wantErr:   true,
		},
		{
			name:      "invalid port",
			input:     "example.com:invalid/http",
			groupName: "",
			isPrimary: false,
			wantErr:   true,
		},
		{
			name:      "invalid host format",
			input:     "example.com/http",
			groupName: "",
			isPrimary: false,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ParseDNSBackend(tt.input, tt.groupName, tt.isPrimary)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDNSBackend() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			if cfg.Domain != tt.domain {
				t.Errorf("ParseDNSBackend() domain = %v, want %v", cfg.Domain, tt.domain)
			}
			if cfg.Protocol != tt.protocol {
				t.Errorf("ParseDNSBackend() protocol = %v, want %v", cfg.Protocol, tt.protocol)
			}
			if cfg.Port != tt.port {
				t.Errorf("ParseDNSBackend() port = %v, want %v", cfg.Port, tt.port)
			}
			if cfg.IsPrimary != tt.isPrimary {
				t.Errorf("ParseDNSBackend() isPrimary = %v, want %v", cfg.IsPrimary, tt.isPrimary)
			}
			if cfg.GroupName != tt.groupName {
				t.Errorf("ParseDNSBackend() groupName = %v, want %v", cfg.GroupName, tt.groupName)
			}
		})
	}
}

func TestFindAddedRemovedItems(t *testing.T) {
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