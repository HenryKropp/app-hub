package handler

import (
	"net/http"
	"testing"
)

func TestSetupLoginLogout(t *testing.T) {
	c := newClient(t, false)

	var status struct {
		NeedsSetup bool        `json:"needs_setup"`
		Person     *personJSON `json:"person"`
	}
	c.do("GET", "/api/auth/status", nil, &status)
	if !status.NeedsSetup || status.Person != nil {
		t.Fatalf("empty hub status = %+v", status)
	}

	admin := setupAdmin(t, c)
	if admin.RoleName != "Administrador" || admin.Email != "henry@alis.com" {
		t.Fatalf("admin = %+v", admin)
	}
	c.do("GET", "/api/auth/status", nil, &status)
	if status.NeedsSetup || status.Person == nil || status.Person.ID != admin.ID {
		t.Fatalf("after setup status = %+v", status)
	}

	// Setup works only once.
	if code := c.fresh().do("POST", "/api/auth/setup", map[string]string{"name": "X", "email": "x@x.com", "password": "12345678"}, nil); code != http.StatusConflict {
		t.Errorf("second setup = %d", code)
	}

	other := c.fresh()
	tests := []struct {
		name     string
		email    string
		password string
		want     int
	}{
		{"wrong password", "henry@alis.com", "errada-123", http.StatusUnauthorized},
		{"unknown email", "ninguem@alis.com", "senha-forte-1", http.StatusUnauthorized},
		{"right password, any case", "HENRY@alis.com", "senha-forte-1", http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if code := other.do("POST", "/api/auth/login", map[string]string{"email": tt.email, "password": tt.password}, nil); code != tt.want {
				t.Errorf("status = %d, want %d", code, tt.want)
			}
		})
	}

	if code := other.do("GET", "/api/people", nil, nil); code != http.StatusOK {
		t.Fatalf("people after login = %d", code)
	}
	other.do("POST", "/api/auth/logout", nil, nil)
	if code := other.do("GET", "/api/people", nil, nil); code != http.StatusUnauthorized {
		t.Errorf("people after logout = %d", code)
	}
}

func TestSetupValidation(t *testing.T) {
	c := newClient(t, false)
	if code := c.do("POST", "/api/auth/setup", map[string]string{"name": "H", "email": "h@x.com", "password": "curta"}, nil); code != http.StatusBadRequest {
		t.Errorf("short password = %d", code)
	}
}

func TestDevLoginAbsentWithoutDevMode(t *testing.T) {
	prod := newClient(t, false)
	if code := prod.do("POST", "/api/dev/login", nil, nil); code == http.StatusOK {
		t.Fatal("dev login must not exist without DEV_MODE")
	}
}

func TestDevLoginInDevMode(t *testing.T) {
	dev := newClient(t, true)
	var p personJSON
	if code := dev.do("POST", "/api/dev/login", nil, &p); code != http.StatusOK || p.RoleName != "Administrador" {
		t.Fatalf("dev login = %d %+v", code, p)
	}
	if code := dev.do("GET", "/api/apps", nil, nil); code != http.StatusOK {
		t.Errorf("apps after dev login = %d", code)
	}
}

func TestPeopleManagement(t *testing.T) {
	c := newClient(t, false)
	setupAdmin(t, c)

	tests := []struct {
		name string
		body map[string]string
		want int
	}{
		{"creator", map[string]string{"name": "Ana", "email": "ana@alis.com", "password": "senha-ana-1", "role": "criador"}, http.StatusCreated},
		{"user", map[string]string{"name": "Bruno", "email": "bruno@alis.com", "password": "senha-bruno", "role": "usuario"}, http.StatusCreated},
		{"duplicate email", map[string]string{"name": "Ana 2", "email": "ANA@alis.com", "password": "senha-ana-2", "role": "usuario"}, http.StatusConflict},
		{"invalid role", map[string]string{"name": "C", "email": "c@alis.com", "password": "senha-c-123", "role": "chefe"}, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if code := c.do("POST", "/api/people", tt.body, nil); code != tt.want {
				t.Errorf("status = %d, want %d", code, tt.want)
			}
		})
	}

	var people []personJSON
	c.do("GET", "/api/people", nil, &people)
	if len(people) != 3 {
		t.Fatalf("people = %+v", people)
	}

	// A non-admin cannot add people.
	bruno := c.fresh()
	bruno.do("POST", "/api/auth/login", map[string]string{"email": "bruno@alis.com", "password": "senha-bruno"}, nil)
	if code := bruno.do("POST", "/api/people", map[string]string{"name": "D", "email": "d@alis.com", "password": "senha-d-123", "role": "admin"}, nil); code != http.StatusForbidden {
		t.Errorf("non-admin create = %d", code)
	}
}
