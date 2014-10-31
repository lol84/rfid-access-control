package main

import (
	"encoding/csv"
	"log"
	"strings"
	"time"
)

type Level string

const (
	LevelLegacy = Level("legacy") // Legacy gate
	LevelUser   = Level("user")
	LevelMember = Level("member")
)

const (
	// Cards that don't have a name or contact info assigned to them are
	// only valid for a limited period, as it otherwise it hard to find
	// the right code if it is stolen/lost or needs revocation.
	// Thus, these will expire automatically.
	//
	// Cards, that have been registered via the LCD ui will not have contact info
	// so they need to be renewed regularly or someone has to simply add contact
	// info to make them valid permanently.
	ValidityPeriodAnonymousCards = 30 * 24 * time.Hour
)

// Note: all Codes are stores as hashAuthCode() defined in authenticator.go
type User struct {
	// Name of user.
	// - Can be empty for time-limited anonymous codes
	// - Members should have a name they go by and can be recognized by
	//   others.
	// - Longer term tokens should also have a name to be able to do
	//   revocations on lost/stolen tokens or excluded visitors.
	Name        string    // Name to go by in the space (not necessarily real-name)
	ContactInfo string    // Way to contact user (if set, should be unique)
	UserLevel   Level     // Level of access
	Sponsors    []string  // A list of (hashed) sponsor codes adding/updating
	ValidFrom   time.Time // E.g. for temporary classes pin
	ValidTo     time.Time // for anonymous tokens, day visitors or temp PIN
	Codes       []string  // List of (hashed) codes associated with user
}

// User CSV
// Fields are stored in the sequence as they appear in the struct, with arrays
// being represented as semicolon separated lists.
// Create a new user read from a CSV reader
func NewUserFromCSV(reader *csv.Reader) (user *User, result_err error) {
	line, err := reader.Read()
	if err != nil {
		return nil, err
	}
	if len(line) != 7 {
		// TODO: add legacy transformation.
		return nil, nil
	}
	// comment
	if strings.TrimSpace(line[0])[0] == '#' {
		return nil, nil
	}
	// TODO: not sure if this does proper locale matching
	ValidFrom, _ := time.Parse("2006-01-02 15:04", line[4])
	ValidTo, _ := time.Parse("2006-01-02 15:04", line[5])
	return &User{
			Name:        line[0],
			ContactInfo: line[1],
			UserLevel:   Level(line[2]),
			Sponsors:    strings.Split(line[3], ";"),
			ValidFrom:   ValidFrom, // field 4
			ValidTo:     ValidTo,   // field 5
			Codes:       strings.Split(line[6], ";")},
		nil
}

func (user *User) WriteCSV(writer *csv.Writer) {
	var fields []string = make([]string, 7)
	fields[0] = user.Name
	fields[1] = user.ContactInfo
	fields[2] = string(user.UserLevel)
	fields[3] = strings.Join(user.Sponsors, ";")
	if !user.ValidFrom.IsZero() {
		fields[4] = user.ValidFrom.Format("2006-01-02 15:04")
	}
	if !user.ValidTo.IsZero() {
		fields[5] = user.ValidTo.Format("2006-01-02 15:04")
	}
	fields[6] = strings.Join(user.Codes, ";")
	writer.Write(fields)
}

// We regard a user to be able to contact if they have a name and contact data
func (user *User) HasContactInfo() bool {
	// Names that start with '<' are auto-generated by
	// the LCD-frontend, so are _not_ considered 'has a name'
	return user != nil &&
		user.Name != "" && user.Name[0] != '<' &&
		user.ContactInfo != ""
}

func (user *User) InValidityPeriod(now time.Time) bool {
	expires := user.ExpiryDate(now)
	return (user.ValidFrom.IsZero() || user.ValidFrom.Before(now)) &&
		(expires.IsZero() || expires.After(now))
}

// Return when code expires. If the returned date IsZero(), there is no limit.
// Even if there is no explicit user.ValidTo
// limited when there is no contact info 30 days after creation
func (user *User) ExpiryDate(now time.Time) time.Time {
	result := user.ValidTo
	if !user.HasContactInfo() {
		if user.ValidFrom.IsZero() {
			log.Println("No start-date for temp code.")
			return now.Add(-24 * time.Hour) // in the past
		}
		anonLimit := user.ValidFrom.Add(ValidityPeriodAnonymousCards)
		if result.IsZero() || anonLimit.Before(result) {
			result = anonLimit
		}
	}
	return user.ValidTo
}

// Set the auth code to some value (should probably be add-auth-code)
// Returns true if code is long enough to meet criteria.
func (user *User) SetAuthCode(code string) bool {
	if !hasMinimalCodeRequirements(code) {
		return false
	}
	user.Codes = []string{hashAuthCode(code)}
	return true
}
