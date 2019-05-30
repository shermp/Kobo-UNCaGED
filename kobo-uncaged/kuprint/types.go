// Copyright 2019 Sherman Perry

// This file is part of Kobo UNCaGED.

// Kobo UNCaGED is free software: you can redistribute it and/or modify
// it under the terms of the Affero GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// Kobo UNCaGED is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.

// You should have received a copy of the GNU Affero General Public License
// along with Kobo UNCaGED.  If not, see <https://www.gnu.org/licenses/>.

package kuprint

// MboxSection is a constant which determines which section of the
// message box to print to
type MboxSection int
// Definition of MboxSection constants
const (
	Header MboxSection = iota
	Body
	Footer
)

// Printer provides functionality to display messages to the user
type Printer interface {
	Println(section MboxSection, a ...interface{}) (n int, err error)
	Close()
}