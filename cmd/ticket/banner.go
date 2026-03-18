package main

import (
	"crypto/rand"
	"math/big"
	"strings"
)

var bannerWords = []string{"TICKET", "TKT", "TCKT", "TKET", "TICKT"}

var bannerGlyphs = map[rune][]string{
	'T': {
		"TTTTTTT",
		"   T   ",
		"   T   ",
		"   T   ",
		"   T   ",
		"   T   ",
	},
	'I': {
		"IIIIIII",
		"   I   ",
		"   I   ",
		"   I   ",
		"   I   ",
		"IIIIIII",
	},
	'C': {
		" CCCCC ",
		"CC   CC",
		"CC     ",
		"CC     ",
		"CC   CC",
		" CCCCC ",
	},
	'K': {
		"KK   KK",
		"KK  KK ",
		"KKKKK  ",
		"KK KK  ",
		"KK  KK ",
		"KK   KK",
	},
	'E': {
		"EEEEEEE",
		"EE     ",
		"EEEEE  ",
		"EEEEE  ",
		"EE     ",
		"EEEEEEE",
	},
}

var bannerColors = []string{
	"\x1b[31m",
	"\x1b[33m",
	"\x1b[32m",
	"\x1b[36m",
	"\x1b[34m",
	"\x1b[35m",
}

func renderBanner() string {
	lines := bannerLines(selectBannerWord())
	var b strings.Builder
	for i, line := range lines {
		color := bannerColors[i%len(bannerColors)]
		b.WriteString(color)
		b.WriteString(line)
		b.WriteString("\x1b[0m\n")
	}
	b.WriteString("\n")
	return b.String()
}

func randomBannerWord() string {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(bannerWords))))
	if err != nil {
		return bannerWords[0]
	}
	return bannerWords[n.Int64()]
}

func bannerLines(word string) []string {
	rows := make([]string, 6)
	upper := strings.ToUpper(strings.TrimSpace(word))
	for _, char := range upper {
		glyph, ok := bannerGlyphs[char]
		if !ok {
			continue
		}
		for i := range rows {
			if rows[i] != "" {
				rows[i] += "  "
			}
			rows[i] += glyph[i]
		}
	}
	return rows
}
