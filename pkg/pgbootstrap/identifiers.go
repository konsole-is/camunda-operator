/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pgbootstrap

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
)

// identifierPattern is the accepted shape of every SQL identifier that this
// package interpolates into DDL: a lowercase PostgreSQL identifier of at most
// 63 characters. It matches the databaseName schema pattern of the Database
// CRD.
var identifierPattern = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)

// validateIdentifier rejects any name that is not a plain lowercase PostgreSQL
// identifier. Quoting alone makes hostile input safe, but it also accepts names
// that the CRD schema forbids. Validation keeps the contract of the SQL layer
// as strict as the contract of the API.
func validateIdentifier(name string) error {
	if !identifierPattern.MatchString(name) {
		return fmt.Errorf("identifier %q must match %s", name, identifierPattern)
	}

	return nil
}

// quoteIdentifier returns name double-quoted for interpolation into DDL. Pass
// name through validateIdentifier first.
func quoteIdentifier(name string) string {
	return pgx.Identifier{name}.Sanitize()
}

// quoteLiteral returns s as a PostgreSQL string literal that is safe to
// interpolate into DDL statements that cannot take bind parameters, such as
// ALTER ROLE ... PASSWORD. It doubles single quotes. If s contains a
// backslash, the literal uses the escape string syntax of PostgreSQL (an E
// prefix) with backslashes escaped. The result is then correct for any
// standard_conforming_strings setting of the server.
func quoteLiteral(s string) string {
	quoted := strings.ReplaceAll(s, "'", "''")

	if strings.Contains(quoted, `\`) {
		return ` E'` + strings.ReplaceAll(quoted, `\`, `\\`) + `'`
	}

	return "'" + quoted + "'"
}
