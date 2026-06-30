package license

// declaredLicenseAliases maps common non-SPDX declared license strings to their
// canonical SPDX identifier. Keys are lower-cased; the Normalizer lowercases its
// input before lookup. The curated map in NewNormalizer takes precedence on any
// overlap.
//
// The alias set is adapted from the public SPDX declared/simple license mapping
// data maintained by the OSS Review Toolkit (Apache-2.0 licensed):
// https://github.com/oss-review-toolkit/ort/tree/main/utils/spdx/src/main/resources
// Only the string -> SPDX-id facts are used; no ORT code is included.
var declaredLicenseAliases = map[string]string{
	// MIT family
	"mit-license":     "MIT",
	"the mit license": "MIT",
	"mit licence":     "MIT",
	"mit/x11":         "MIT",
	"x11":             "X11",

	// Apache family
	"apache license":              "Apache-2.0",
	"apache license, version 2.0": "Apache-2.0",
	"apache license version 2.0":  "Apache-2.0",
	"apache software license":     "Apache-2.0",
	"apache-2":                    "Apache-2.0",
	"asl 2.0":                     "Apache-2.0",
	"apache public license 2.0":   "Apache-2.0",
	"apache 2":                    "Apache-2.0",
	"apache license 1.1":          "Apache-1.1",
	"apache-1.1":                  "Apache-1.1",
	"apache-1.0":                  "Apache-1.0",

	// BSD family
	"bsd license":            "BSD-3-Clause",
	"bsd-3":                  "BSD-3-Clause",
	"bsd 3-clause":           "BSD-3-Clause",
	"new bsd license":        "BSD-3-Clause",
	"modified bsd license":   "BSD-3-Clause",
	"bsd-2":                  "BSD-2-Clause",
	"simplified bsd license": "BSD-2-Clause",
	"freebsd":                "BSD-2-Clause",
	"bsd 2-clause":           "BSD-2-Clause",
	"bsd-3-clause-clear":     "BSD-3-Clause-Clear",

	// GPL family
	"gnu gpl":                       "GPL-3.0-only",
	"gnu general public license":    "GPL-3.0-only",
	"gnu general public license v2": "GPL-2.0-only",
	"gnu general public license v3": "GPL-3.0-only",
	"gpl3":                          "GPL-3.0-only",
	"gpl2":                          "GPL-2.0-only",
	"gplv3+":                        "GPL-3.0-or-later",
	"gplv2+":                        "GPL-2.0-or-later",

	// LGPL family
	"gnu lgpl":                          "LGPL-3.0-only",
	"gnu lesser general public license": "LGPL-3.0-only",
	"lgpl3":                             "LGPL-3.0-only",
	"lgpl2":                             "LGPL-2.1-only",
	"lgplv3+":                           "LGPL-3.0-or-later",
	"lgplv2.1+":                         "LGPL-2.1-or-later",

	// AGPL family
	"gnu agpl": "AGPL-3.0-only",
	"agplv3+":  "AGPL-3.0-or-later",

	// Mozilla
	"mpl":                        "MPL-2.0",
	"mpl 2.0":                    "MPL-2.0",
	"mpl-1.1":                    "MPL-1.1",
	"mpl-1.0":                    "MPL-1.0",
	"mozilla public license 2.0": "MPL-2.0",

	// Eclipse
	"eclipse public license - v 1.0": "EPL-1.0",
	"eclipse public license - v 2.0": "EPL-2.0",
	"eclipse public license v2.0":    "EPL-2.0",

	// CDDL
	"common development and distribution license":     "CDDL-1.0",
	"common development and distribution license 1.0": "CDDL-1.0",
	"common development and distribution license 1.1": "CDDL-1.1",

	// Creative Commons
	"cc0":          "CC0-1.0",
	"cc-by-4.0":    "CC-BY-4.0",
	"cc-by-3.0":    "CC-BY-3.0",
	"cc-by-sa-4.0": "CC-BY-SA-4.0",

	// Public domain / permissive
	"public domain":   "Public-Domain",
	"the unlicense":   "Unlicense",
	"zero-clause bsd": "0BSD",
	"do what the f*ck you want to public license": "WTFPL",

	// Other common
	"the artistic license 2.0":           "Artistic-2.0",
	"artistic license 2.0":               "Artistic-2.0",
	"artistic-1.0":                       "Artistic-1.0",
	"python software foundation license": "PSF-2.0",
	"psf":                                "PSF-2.0",
	"python-2.0":                         "Python-2.0",
	"ruby license":                       "Ruby",
	"the ruby license":                   "Ruby",
	"openssl license":                    "OpenSSL",
	"microsoft public license":           "MS-PL",
	"ms-pl":                              "MS-PL",
	"microsoft reciprocal license":       "MS-RL",
	"ms-rl":                              "MS-RL",
	"boost software license 1.0":         "BSL-1.0",
	"the boost license":                  "BSL-1.0",
	"isc license":                        "ISC",
	"the isc license":                    "ISC",
	"zlib license":                       "Zlib",
	"university of illinois/ncsa open source license": "NCSA",
	"ncsa":                        "NCSA",
	"sspl-1.0":                    "SSPL-1.0",
	"server side public license":  "SSPL-1.0",
	"business source license 1.1": "BUSL-1.1",
	"vim license":                 "Vim",
	"the vim license":             "Vim",
	"beerware":                    "Beerware",
}
