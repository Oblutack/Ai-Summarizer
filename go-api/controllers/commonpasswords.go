package controllers

import "strings"

// commonPasswordList are passwords of eight characters or more that are guessed first by anyone attacking an account:
// the ones that top the lists of leaked passwords, their usual variations, keyboard patterns, and names of places and
// things that people pick. A password on this list is refused outright (the check ignores letter case). It is a floor,
// not a promise that everything else is strong: the length rule and the pattern checks below cover the rest.
const commonPasswordList = `
12345678 123456789 1234567890 12345678910 123456789a 12345678a 1234567a a12345678 a1234567 a1b2c3d4 abc12345 abc123456
abcd1234 abcd12345 abcdefgh abcdefg1 abcdefgh1 00000000 11111111 22222222 88888888 12341234 12121212 123123123 123321123
147258369 987654321 963852741 1q2w3e4r 1q2w3e4r5t 1qaz2wsx 1qazxsw2 zaq12wsx qazwsxedc qwertyui qwertyuiop qwerty12
qwerty123 qwerty1234 qwerty12345 qwertyu1 qwe12345 qwe123qwe qwe123456 qweasdzxc 123qweasd asdfghjk asdfghjkl asdf1234
asd12345 asdasd123 asdasdasd zxcvbnm1 zxczxc123 zxcvbnm123 passw0rd passw0rd1 password password1 password12 password123
password1234 password12345 password!1 password1! pa$$word pa$$w0rd p@ssw0rd p@ssword p@ssw0rd1 passpass passpass1
mypassword mypass123 mypassword1 yourpassword newpassword letmein1 letmein12 letmein123 letmein! welcome1 welcome12
welcome123 welcome1234 iloveyou iloveyou1 iloveyou2 iloveyou12 iloveyou123 sunshine sunshine1 princess princess1
football football1 baseball baseball1 basketball superman superman1 trustno1 whatever whatever1 starwars computer
internet changeme changeme1 changeme123 admin123 admin1234 admin12345 administrator root1234 toor1234 guest123
guest1234 test1234 testtest test12345 temp1234 default1 secret12 secret123 hello123 hello1234 helloworld hello12345
monkey123 monkey12 dragon12 dragon123 master12 master123 shadow12 shadow123 killer12 hunter12 soccer12 hockey12
batman12 ninja123 freedom1 pokemon1 minecraft fortnite1 roblox123 google123 facebook1 facebook123 instagram1
michael1 charlie1 jessica1 jennifer1 michelle1 ashley12 nicole12 daniel12 thomas12 matthew1 andrew12 joshua12
robert12 summer12 spring12 winter12 autumn12 january1 february1 liverpool chelsea1 arsenal1 manchester barcelona
realmadrid mercedes18 lozinka1 lozinka12 lozinka123 sifra123 sarajevo1 zagreb123 passwort passwort1 passwort123
motdepasse contrasena contrasena1 inkling1 inkling12 inkling123 summarize summarize1 18atcskd2w 3rjs1la7qe
`

var commonPasswords = func() map[string]bool {
	set := map[string]bool{}
	for _, word := range strings.Fields(commonPasswordList) {
		set[strings.ToLower(word)] = true
	}
	return set
}()

// runs are strings in which typing the next key is the whole idea; a password that is a stretch of one is no secret.
var runs = []string{
	"0123456789", "9876543210", "abcdefghijklmnopqrstuvwxyz", "zyxwvutsrqponmlkjihgfedcba",
	"qwertyuiop", "poiuytrewq", "asdfghjkl", "lkjhgfdsa", "zxcvbnm", "mnbvcxz", "1qaz2wsx3edc4rfv", "qazwsxedcrfv",
}

// isTooSimple catches the passwords no list can: one character repeated, a short pattern repeated ("abababab",
// "12341234"), and a stretch of digits, letters or one keyboard row ("87654321", "klmnopqr", "asdfghjk").
func isTooSimple(password string) bool {
	lower := strings.ToLower(password)
	runes := []rune(lower)
	if len(runes) == 0 {
		return true
	}
	// one character, or a short pattern, over and over
	for size := 1; size <= len(runes)/2 && size <= 4; size++ {
		if len(runes)%size != 0 {
			continue
		}
		repeated := true
		for i := size; i < len(runes); i++ {
			if runes[i] != runes[i-size] {
				repeated = false
				break
			}
		}
		if repeated {
			return true
		}
	}
	for _, run := range runs {
		if strings.Contains(run, lower) {
			return true
		}
	}
	return false
}

// containsLocalPart says whether the password has the part of the email before the @ in it ("alice2024" for
// alice@example.com): the first thing guessed once someone knows whose account it is. Short names are ignored: "jo"
// is in too many ordinary passwords to mean anything.
func containsLocalPart(password, email string) bool {
	local, _, found := strings.Cut(strings.ToLower(email), "@")
	return found && len([]rune(local)) >= 4 && strings.Contains(strings.ToLower(password), local)
}
