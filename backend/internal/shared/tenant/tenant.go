// Package tenant derives every technical name of a company's shared apps
// (containers, images, databases, folders, addresses) from one short key, so
// the same app id in two companies never shares anything. Principal keeps the
// empty key and exactly the names it had before companies existed.
package tenant

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
)

// imagePrefix mirrors engine.ImagePrefix; the engine package tests that they match
// (tenant cannot import engine without a cycle).
const imagePrefix = "localhost/ziv/"

// Tenant is a company as its shared apps see it.
type Tenant struct{ Slug, Key string }

// Principal is the main company: empty key, today's names.
var Principal = Tenant{Slug: "principal"}

var schemaRe = regexp.MustCompile(`^tenant_([0-9a-f]{24})$`)

// KeyOf turns a company's schema name into its key: "public" is principal (""),
// tenant_<24 hex> becomes "w" plus the first 11 hex characters.
func KeyOf(schema string) (string, error) {
	if schema == "public" {
		return "", nil
	}
	m := schemaRe.FindStringSubmatch(schema)
	if m == nil {
		return "", errors.New("tenant: unexpected schema name " + schema)
	}
	return "w" + m[1][:11], nil
}

// Principal reports whether this is the main company.
func (t Tenant) Principal() bool { return t.Key == "" }

// AppsDomain is the domain the company's app addresses live under.
func (t Tenant) AppsDomain(base string) string {
	if t.Principal() {
		return base
	}
	return t.Slug + "." + base
}

// DataDir is the company's folder for stage, apps, backups and code.
func (t Tenant) DataDir(root string) string {
	if t.Principal() {
		return root
	}
	return filepath.Join(root, "workspaces", t.Key)
}

// DBPrefix starts every app database and role name of the company.
func (t Tenant) DBPrefix() string {
	if t.Principal() {
		return "app_"
	}
	return "app_" + t.Key + "_"
}

func (t Tenant) namePart() string {
	if t.Principal() {
		return ""
	}
	return t.Key + "-"
}

// ContainerName names an app's box; test boxes carry "teste".
func (t Tenant) ContainerName(appSlug string, depID int64, test bool) string {
	if test {
		return fmt.Sprintf("ziv-%s%s-teste-%d", t.namePart(), appSlug, depID)
	}
	return fmt.Sprintf("ziv-%s%s-%d", t.namePart(), appSlug, depID)
}

// ImageTag names the image built for a deployment.
func (t Tenant) ImageTag(appSlug string, depID int64) string {
	if t.Principal() {
		return fmt.Sprintf("%s%s:%d", imagePrefix, appSlug, depID)
	}
	return fmt.Sprintf("%s%s/%s:%d", imagePrefix, t.Key, appSlug, depID)
}
