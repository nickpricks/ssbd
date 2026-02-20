package core

import (
	"strings"
	"sync"
)

// Common passwords — a curated subset for embedding. In production, this would
// be a much larger compressed list loaded via //go:embed. For now we embed the
// top ~200 most common passwords to keep the binary lean during development.
// This list will be expanded with SecLists data in a future update.
var commonPasswordsList = []string{
	"123456", "password", "12345678", "qwerty", "123456789",
	"12345", "1234", "111111", "1234567", "dragon",
	"123123", "baseball", "abc123", "football", "monkey",
	"letmein", "shadow", "master", "666666", "qwertyuiop",
	"123321", "mustang", "1234567890", "michael", "654321",
	"superman", "1qaz2wsx", "7777777", "121212", "000000",
	"qazwsx", "123qwe", "killer", "trustno1", "jordan",
	"jennifer", "zxcvbnm", "asdfgh", "hunter", "buster",
	"soccer", "harley", "batman", "andrew", "tigger",
	"sunshine", "iloveyou", "2000", "charlie", "robert",
	"thomas", "hockey", "ranger", "daniel", "starwars",
	"klaster", "112233", "george", "computer", "michelle",
	"jessica", "pepper", "1111", "zxcvbn", "555555",
	"11111111", "131313", "freedom", "777777", "pass",
	"maggie", "159753", "aaaaaa", "ginger", "princess",
	"joshua", "cheese", "amanda", "summer", "love",
	"ashley", "nicole", "chelsea", "biteme", "matthew",
	"access", "yankees", "987654321", "dallas", "austin",
	"thunder", "taylor", "matrix", "minecraft", "william",
	"corvette", "hello", "martin", "heather", "secret",
	"fucker", "merlin", "diamond", "1234qwer", "gfhjkm",
	"hammer", "silver", "222222", "88888888", "anthony",
	"justin", "test", "bailey", "q1w2e3r4t5", "patrick",
	"internet", "scooter", "orange", "11111", "golfer",
	"cookie", "richard", "samantha", "bigdog", "guitar",
	"jackson", "whatever", "mickey", "chicken", "sparky",
	"snoopy", "maverick", "phoenix", "camaro", "peanut",
	"morgan", "welcome", "falcon", "cowboy", "ferrari",
	"samsung", "andrea", "smokey", "steelers", "joseph",
	"mercedes", "dakota", "arsenal", "eagles", "melissa",
	"boomer", "booboo", "spider", "nascar", "monster",
	"tigers", "yellow", "xxxxxx", "123456a", "golf",
	"buddy", "edward", "genesis", "hannah", "jessie",
	"natasha", "doctor", "titanic", "liverpool", "banana",
	"chester", "joshua1", "amanda1", "summer1", "1q2w3e4r",
	"admin", "passw0rd", "password1", "password123", "letmein1",
	"welcome1", "monkey1", "master1", "qwerty123", "login",
	"abc1234", "starwars1", "administrator", "passwd", "test1",
	"test123", "p@ssword", "p@ss", "p@ssw0rd", "pa$$word",
}

var (
	commonPasswordsOnce sync.Once
	commonPasswordsSet  map[string]struct{}
)

func loadCommonPasswords() {
	commonPasswordsOnce.Do(func() {
		commonPasswordsSet = make(map[string]struct{}, len(commonPasswordsList))
		for _, pw := range commonPasswordsList {
			commonPasswordsSet[strings.ToLower(pw)] = struct{}{}
		}
	})
}

// isCommonPassword checks whether the given password exists in the common passwords list.
func isCommonPassword(password string) bool {
	loadCommonPasswords()
	_, found := commonPasswordsSet[strings.ToLower(password)]
	return found
}
