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

// identifierPattern is the accepted shape for every SQL identifier this
// package interpolates into DDL: lowercase PostgreSQL identifiers of at most
// 63 characters, matching the Database CRD's databaseName schema pattern.
var identifierPattern = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)

// validateIdentifier rejects any name that is not a plain lowercase PostgreSQL
// identifier. Quoting alone would make hostile input safe but silently accept
// names the CRD schema forbids; validation keeps the SQL layer's contract as
// strict as the API's.
func validateIdentifier(name string) error {
	if !identifierPattern.MatchString(name) {
		return fmt.Errorf("identifier %q must match %s", name, identifierPattern)
	}

	return nil
}

// quoteIdentifier returns name double-quoted for interpolation into DDL.
// Callers must have passed name through validateIdentifier first.
func quoteIdentifier(name string) string {
	return pgx.Identifier{name}.Sanitize()
}

// quoteLiteral returns s as a PostgreSQL string literal safe to interpolate
// into DDL statements that cannot take bind parameters, such as ALTER ROLE
// ... PASSWORD. Single quotes are doubled; when s contains a backslash the
// literal uses the E'' form with backslashes escaped, so the result is correct
// regardless of the server's standard_conforming_strings setting.
func quoteLiteral(s string) string {
	quoted := strings.ReplaceAll(s, "'", "''")

	if strings.Contains(quoted, `\`) {
		return ` E'` + strings.ReplaceAll(quoted, `\`, `\\`) + `'`
	}

	return "'" + quoted + "'"
}
