package geoip

import (
	"fmt"
	"github.com/valyala/fasthttp"
	"gitlab.adtelligent.com/common/shared/log"
	"net"
	"strconv"
	"strings"
)

func HTTPHandlerDictCountries(ctx *fasthttp.RequestCtx) {
	ctx.SetContentType("text/csv")
	for key := range cByA3 {
		c := cByA3[key]
		eu := 0
		if c.IsEU() {
			eu = 1
		}
		_, _ = fmt.Fprintf(ctx, "%d,%s,%s,%v\n", c.ID, c.Alfa2, c.Alfa3, eu)
	}
}

type Country struct {
	ID    uint32
	Alfa2 string
	// TBD - can be empty - A2,O1,AP,EU ,ZZ,A1
	Alfa3 string
}

var a3ByA2 = func() map[string]string {
	s :=
		`AF	AFG
AX	ALA
AL	ALB
DZ	DZA
AS	ASM
AD	AND
AO	AGO
AI	AIA
AQ	ATA
AG	ATG
AR	ARG
AM	ARM
AW	ABW
AU	AUS
AT	AUT
AZ	AZE
BS	BHS
BH	BHR
BD	BGD
BB	BRB
BY	BLR
BE	BEL
BZ	BLZ
BJ	BEN
BM	BMU
BT	BTN
BO	BOL
BQ	BES
BA	BIH
BW	BWA
BV	BVT
BR	BRA
IO	IOT
BN	BRN
BG	BGR
BF	BFA
BI	BDI
CV	CPV
KH	KHM
CM	CMR
CA	CAN
KY	CYM
CF	CAF
TD	TCD
CL	CHL
CN	CHN
CX	CXR
CC	CCK
CO	COL
KM	COM
CG	COG
CD	COD
CK	COK
CR	CRI
CI	CIV
HR	HRV
CU	CUB
CW	CUW
CY	CYP
CZ	CZE
DK	DNK
DJ	DJI
DM	DMA
DO	DOM
EC	ECU
EG	EGY
SV	SLV
GQ	GNQ
ER	ERI
EE	EST
ET	ETH
FK	FLK
FO	FRO
FJ	FJI
FI	FIN
FR	FRA
GF	GUF
PF	PYF
TF	ATF
GA	GAB
GM	GMB
GE	GEO
DE	DEU
GH	GHA
GI	GIB
GR	GRC
GL	GRL
GD	GRD
GP	GLP
GU	GUM
GT	GTM
GG	GGY
GN	GIN
GW	GNB
GY	GUY
HT	HTI
HM	HMD
VA	VAT
HN	HND
HK	HKG
HU	HUN
IS	ISL
IN	IND
ID	IDN
IR	IRN
IQ	IRQ
IE	IRL
IM	IMN
IL	ISR
IT	ITA
JM	JAM
JP	JPN
JE	JEY
JO	JOR
KZ	KAZ
KE	KEN
KI	KIR
KP	PRK
KR	KOR
KW	KWT
KG	KGZ
LA	LAO
LV	LVA
LB	LBN
LS	LSO
LR	LBR
LY	LBY
LI	LIE
LT	LTU
LU	LUX
MO	MAC
MK	MKD
MG	MDG
MW	MWI
MY	MYS
MV	MDV
ML	MLI
MT	MLT
MH	MHL
MQ	MTQ
MR	MRT
MU	MUS
YT	MYT
MX	MEX
FM	FSM
MD	MDA
MC	MCO
MN	MNG
ME	MNE
MS	MSR
MA	MAR
MZ	MOZ
MM	MMR
NA	NAM
NR	NRU
NP	NPL
NL	NLD
NC	NCL
NZ	NZL
NI	NIC
NE	NER
NG	NGA
NU	NIU
NF	NFK
MP	MNP
NO	NOR
OM	OMN
PK	PAK
PW	PLW
PS	PSE
PA	PAN
PG	PNG
PY	PRY
PE	PER
PH	PHL
PN	PCN
PL	POL
PT	PRT
PR	PRI
QA	QAT
RE	REU
RO	ROU
RU	RUS
RW	RWA
BL	BLM
SH	SHN
KN	KNA
LC	LCA
MF	MAF
PM	SPM
VC	VCT
WS	WSM
SM	SMR
ST	STP
SA	SAU
SN	SEN
RS	SRB
SC	SYC
SL	SLE
SG	SGP
SX	SXM
SK	SVK
SI	SVN
SB	SLB
SO	SOM
ZA	ZAF
GS	SGS
ES	ESP
LK	LKA
SD	SDN
SR	SUR
SJ	SJM
SZ	SWZ
SE	SWE
CH	CHE
SY	SYR
TW	TWN
TJ	TJK
TZ	TZA
TH	THA
TL	TLS
TG	TGO
TK	TKL
TO	TON
TT	TTO
TN	TUN
TR	TUR
TM	TKM
TC	TCA
TV	TUV
UG	UGA
UA	UKR
AE	ARE
GB	GBR
US	USA
UM	UMI
UY	URY
UZ	UZB
VU	VUT
VE	VEN
VN	VNM
VG	VGB
VI	VIR
WF	WLF
EH	ESH
YE	YEM
ZM	ZMB
ZW	ZWE
ZZ	ZZZ
SS	SSD`

	m := make(map[string]string)
	for _, line := range strings.Split(s, "\n") {
		tmp := strings.Split(line, "\t")
		if len(tmp) != 2 {
			log.Panicf("cannot find \\t delimiter on line %q", line)
		}
		a2 := strings.TrimSpace(tmp[0])
		a3 := strings.TrimSpace(tmp[1])
		m[a2] = a3
	}
	return m
}()

var cByA2, cByA3 = func() (map[string]*Country, map[string]*Country) {
	s :=
		`|         1 | A1           | Anonymous Proxy                              |
|         2 | A2           | Satellite Provider                           |
|         3 | O1           | Other Country                                |
|         4 | AD           | Andorra                                      |
|         5 | AE           | United Arab Emirates                         |
|         6 | AF           | Afghanistan                                  |
|         7 | AG           | Antigua and Barbuda                          |
|         8 | AI           | Anguilla                                     |
|         9 | AL           | Albania                                      |
|        10 | AM           | Armenia                                      |
|        11 | AO           | Angola                                       |
|        12 | AP           | Asia/Pacific Region                          |
|        13 | AQ           | Antarctica                                   |
|        14 | AR           | Argentina                                    |
|        15 | AS           | American Samoa                               |
|        16 | AT           | Austria                                      |
|        17 | AU           | Australia                                    |
|        18 | AW           | Aruba                                        |
|        19 | AX           | Aland Islands                                |
|        20 | AZ           | Azerbaijan                                   |
|        21 | BA           | Bosnia and Herzegovina                       |
|        22 | BB           | Barbados                                     |
|        23 | BD           | Bangladesh                                   |
|        24 | BE           | Belgium                                      |
|        25 | BF           | Burkina Faso                                 |
|        26 | BG           | Bulgaria                                     |
|        27 | BH           | Bahrain                                      |
|        28 | BI           | Burundi                                      |
|        29 | BJ           | Benin                                        |
|        30 | BL           | Saint Bartelemey                             |
|        31 | BM           | Bermuda                                      |
|        32 | BN           | Brunei Darussalam                            |
|        33 | BO           | Bolivia                                      |
|        34 | BQ           | Bonaire, Saint Eustatius and Saba            |
|        35 | BR           | Brazil                                       |
|        36 | BS           | Bahamas                                      |
|        37 | BT           | Bhutan                                       |
|        38 | BV           | Bouvet Island                                |
|        39 | BW           | Botswana                                     |
|        40 | BY           | Belarus                                      |
|        41 | BZ           | Belize                                       |
|        42 | CA           | Canada                                       |
|        43 | CC           | Cocos (Keeling) Islands                      |
|        44 | CD           | Congo, The Democratic Republic of the        |
|        45 | CF           | Central African Republic                     |
|        46 | CG           | Congo                                        |
|        47 | CH           | Switzerland                                  |
|        48 | CI           | Cote d'Ivoire                                |
|        49 | CK           | Cook Islands                                 |
|        50 | CL           | Chile                                        |
|        51 | CM           | Cameroon                                     |
|        52 | CN           | China                                        |
|        53 | CO           | Colombia                                     |
|        54 | CR           | Costa Rica                                   |
|        55 | CU           | Cuba                                         |
|        56 | CV           | Cape Verde                                   |
|        57 | CW           | Curacao                                      |
|        58 | CX           | Christmas Island                             |
|        59 | CY           | Cyprus                                       |
|        60 | CZ           | Czech Republic                               |
|        61 | DE           | Germany                                      |
|        62 | DJ           | Djibouti                                     |
|        63 | DK           | Denmark                                      |
|        64 | DM           | Dominica                                     |
|        65 | DO           | Dominican Republic                           |
|        66 | DZ           | Algeria                                      |
|        67 | EC           | Ecuador                                      |
|        68 | EE           | Estonia                                      |
|        69 | EG           | Egypt                                        |
|        70 | EH           | Western Sahara                               |
|        71 | ER           | Eritrea                                      |
|        72 | ES           | Spain                                        |
|        73 | ET           | Ethiopia                                     |
|        74 | EU           | Europe                                       |
|        75 | FI           | Finland                                      |
|        76 | FJ           | Fiji                                         |
|        77 | FK           | Falkland Islands (Malvinas)                  |
|        78 | FM           | Micronesia, Federated States of              |
|        79 | FO           | Faroe Islands                                |
|        80 | FR           | France                                       |
|        81 | GA           | Gabon                                        |
|        82 | GB           | United Kingdom                               |
|        83 | GD           | Grenada                                      |
|        84 | GE           | Georgia                                      |
|        85 | GF           | French Guiana                                |
|        86 | GG           | Guernsey                                     |
|        87 | GH           | Ghana                                        |
|        88 | GI           | Gibraltar                                    |
|        89 | GL           | Greenland                                    |
|        90 | GM           | Gambia                                       |
|        91 | GN           | Guinea                                       |
|        92 | GP           | Guadeloupe                                   |
|        93 | GQ           | Equatorial Guinea                            |
|        94 | GR           | Greece                                       |
|        95 | GS           | South Georgia and the South Sandwich Islands |
|        96 | GT           | Guatemala                                    |
|        97 | GU           | Guam                                         |
|        98 | GW           | Guinea-Bissau                                |
|        99 | GY           | Guyana                                       |
|       100 | HK           | Hong Kong                                    |
|       101 | HM           | Heard Island and McDonald Islands            |
|       102 | HN           | Honduras                                     |
|       103 | HR           | Croatia                                      |
|       104 | HT           | Haiti                                        |
|       105 | HU           | Hungary                                      |
|       106 | ID           | Indonesia                                    |
|       107 | IE           | Ireland                                      |
|       108 | IL           | Israel                                       |
|       109 | IM           | Isle of Man                                  |
|       110 | IN           | India                                        |
|       111 | IO           | British Indian Ocean Territory               |
|       112 | IQ           | Iraq                                         |
|       113 | IR           | Iran, Islamic Republic of                    |
|       114 | IS           | Iceland                                      |
|       115 | IT           | Italy                                        |
|       116 | JE           | Jersey                                       |
|       117 | JM           | Jamaica                                      |
|       118 | JO           | Jordan                                       |
|       119 | JP           | Japan                                        |
|       120 | KE           | Kenya                                        |
|       121 | KG           | Kyrgyzstan                                   |
|       122 | KH           | Cambodia                                     |
|       123 | KI           | Kiribati                                     |
|       124 | KM           | Comoros                                      |
|       125 | KN           | Saint Kitts and Nevis                        |
|       126 | KP           | Korea, Democratic People's Republic of       |
|       127 | KR           | Korea, Republic of                           |
|       128 | KW           | Kuwait                                       |
|       129 | KY           | Cayman Islands                               |
|       130 | KZ           | Kazakhstan                                   |
|       131 | LA           | Lao People's Democratic Republic             |
|       132 | LB           | Lebanon                                      |
|       133 | LC           | Saint Lucia                                  |
|       134 | LI           | Liechtenstein                                |
|       135 | LK           | Sri Lanka                                    |
|       136 | LR           | Liberia                                      |
|       137 | LS           | Lesotho                                      |
|       138 | LT           | Lithuania                                    |
|       139 | LU           | Luxembourg                                   |
|       140 | LV           | Latvia                                       |
|       141 | LY           | Libyan Arab Jamahiriya                       |
|       142 | MA           | Morocco                                      |
|       143 | MC           | Monaco                                       |
|       144 | MD           | Moldova, Republic of                         |
|       145 | ME           | Montenegro                                   |
|       146 | MF           | Saint Martin                                 |
|       147 | MG           | Madagascar                                   |
|       148 | MH           | Marshall Islands                             |
|       149 | MK           | Macedonia                                    |
|       150 | ML           | Mali                                         |
|       151 | MM           | Myanmar                                      |
|       152 | MN           | Mongolia                                     |
|       153 | MO           | Macao                                        |
|       154 | MP           | Northern Mariana Islands                     |
|       155 | MQ           | Martinique                                   |
|       156 | MR           | Mauritania                                   |
|       157 | MS           | Montserrat                                   |
|       158 | MT           | Malta                                        |
|       159 | MU           | Mauritius                                    |
|       160 | MV           | Maldives                                     |
|       161 | MW           | Malawi                                       |
|       162 | MX           | Mexico                                       |
|       163 | MY           | Malaysia                                     |
|       164 | MZ           | Mozambique                                   |
|       165 | NA           | Namibia                                      |
|       166 | NC           | New Caledonia                                |
|       167 | NE           | Niger                                        |
|       168 | NF           | Norfolk Island                               |
|       169 | NG           | Nigeria                                      |
|       170 | NI           | Nicaragua                                    |
|       171 | NL           | Netherlands                                  |
|       172 | NO           | Norway                                       |
|       173 | NP           | Nepal                                        |
|       174 | NR           | Nauru                                        |
|       175 | NU           | Niue                                         |
|       176 | NZ           | New Zealand                                  |
|       177 | OM           | Oman                                         |
|       178 | PA           | Panama                                       |
|       179 | PE           | Peru                                         |
|       180 | PF           | French Polynesia                             |
|       181 | PG           | Papua New Guinea                             |
|       182 | PH           | Philippines                                  |
|       183 | PK           | Pakistan                                     |
|       184 | PL           | Poland                                       |
|       185 | PM           | Saint Pierre and Miquelon                    |
|       186 | PN           | Pitcairn                                     |
|       187 | PR           | Puerto Rico                                  |
|       188 | PS           | Palestinian Territory                        |
|       189 | PT           | Portugal                                     |
|       190 | PW           | Palau                                        |
|       191 | PY           | Paraguay                                     |
|       192 | QA           | Qatar                                        |
|       193 | RE           | Reunion                                      |
|       194 | RO           | Romania                                      |
|       195 | RS           | Serbia                                       |
|       196 | RU           | Russian Federation                           |
|       197 | RW           | Rwanda                                       |
|       198 | SA           | Saudi Arabia                                 |
|       199 | SB           | Solomon Islands                              |
|       200 | SC           | Seychelles                                   |
|       201 | SD           | Sudan                                        |
|       202 | SE           | Sweden                                       |
|       203 | SG           | Singapore                                    |
|       204 | SH           | Saint Helena                                 |
|       205 | SI           | Slovenia                                     |
|       206 | SJ           | Svalbard and Jan Mayen                       |
|       207 | SK           | Slovakia                                     |
|       208 | SL           | Sierra Leone                                 |
|       209 | SM           | San Marino                                   |
|       210 | SN           | Senegal                                      |
|       211 | SO           | Somalia                                      |
|       212 | SR           | Suriname                                     |
|       213 | ST           | Sao Tome and Principe                        |
|       214 | SV           | El Salvador                                  |
|       215 | SX           | Sint Maarten                                 |
|       216 | SY           | Syrian Arab Republic                         |
|       217 | SZ           | Swaziland                                    |
|       218 | TC           | Turks and Caicos Islands                     |
|       219 | TD           | Chad                                         |
|       220 | TF           | French Southern Territories                  |
|       221 | TG           | Togo                                         |
|       222 | TH           | Thailand                                     |
|       223 | TJ           | Tajikistan                                   |
|       224 | TK           | Tokelau                                      |
|       225 | TL           | Timor-Leste                                  |
|       226 | TM           | Turkmenistan                                 |
|       227 | TN           | Tunisia                                      |
|       228 | TO           | Tonga                                        |
|       229 | TR           | Turkey                                       |
|       230 | TT           | Trinidad and Tobago                          |
|       231 | TV           | Tuvalu                                       |
|       232 | TW           | Taiwan                                       |
|       233 | TZ           | Tanzania, United Republic of                 |
|       234 | UA           | Ukraine                                      |
|       235 | UG           | Uganda                                       |
|       236 | UM           | United States Minor Outlying Islands         |
|       237 | US           | United States                                |
|       238 | UY           | Uruguay                                      |
|       239 | UZ           | Uzbekistan                                   |
|       240 | VA           | Holy See (Vatican City State)                |
|       241 | VC           | Saint Vincent and the Grenadines             |
|       242 | VE           | Venezuela                                    |
|       243 | VG           | Virgin Islands, British                      |
|       244 | VI           | Virgin Islands, U.S.                         |
|       245 | VN           | Vietnam                                      |
|       246 | VU           | Vanuatu                                      |
|       247 | WF           | Wallis and Futuna                            |
|       248 | WS           | Samoa                                        |
|       249 | YE           | Yemen                                        |
|       250 | YT           | Mayotte                                      |
|       251 | ZA           | South Africa                                 |
|       252 | ZM           | Zambia                                       |
|       253 | ZW           | Zimbabwe                                     |
|       254 | ZZ           | All countries                                |
|       255 | SS           | South Sudan                                  |
|       256 | XK           | Kosovo                                       |`

	mByA2 := make(map[string]*Country)
	mByA3 := make(map[string]*Country)
	for _, line := range strings.Split(s, "\n") {
		tmp := strings.Split(line, "|")
		a2 := strings.TrimSpace(tmp[2])
		a3 := a3ByA2[a2]
		id, err := strconv.Atoi(strings.TrimSpace(tmp[1]))
		if err != nil {
			log.Panicf("cannot parse country id on line %q: %s", line, err)
		}
		c := &Country{
			ID:    uint32(id),
			Alfa2: a2,
			Alfa3: a3,
		}
		mByA2[a2] = c
		mByA3[a3] = c
	}
	return mByA2, mByA3
}()

func CountryByID(id uint32) *Country {
	for i := range cByA2 {
		c := cByA2[i]
		if c.ID == id {
			return c
		}
	}

	return cByA2["ZZ"]
}

// CountryByCode returns country for the given country code.
//
// The function always returns non-nil result.
func CountryByCode(code string) *Country {
	var c *Country
	if len(code) == 3 {
		c = cByA3[code]
	} else {
		c = cByA2[code]
	}
	if c != nil {
		return c
	}

	// Try uppercase value code.
	codeUpper := strings.ToUpper(code)
	if codeUpper != code {
		return CountryByCode(codeUpper)
	}

	if len(code) > 0 && code[0] >= 'A' && code[0] <= 'Z' {
		log.Errorf("FIXME: unknown country code %q", code)
	}

	return cByA2["ZZ"]
}

// CountryByIP returns country for the given ip.
//
// The function always returns non-nil result.
func CountryByIP(ip net.IP) *Country {
	code := CountryCodeByIP(ip)
	c := cByA2[code]
	if c != nil {
		return c
	}

	log.Errorf("FIXME: unknown country code %q for ip %q", code, ip)
	return cByA2["ZZ"]
}

// CountryIDByIP returns country id for the given ip.
func CountryIDByIP(ip net.IP) uint32 {
	c := CountryByIP(ip)
	return c.ID
}

var usaWC = map[string]struct{}{
	"CA": {},
	"OR": {},
	"WA": {},
	"UT": {},
	"NV": {},
}

func (c *City) Zone() string {
	if c.IsEU() {
		return "EU"
	} else if c.IsUSAWestCost() {
		return "US/WC"
	} else if c.IsUSAEastCost() {
		return "US/EC"
	}

	return fmt.Sprintf("OTHER/%s", c.Country.Alfa2)
}

func (c *City) IsEU() bool {
	return c.Country.IsEU()
}

func (c *City) IsUSAWestCost() bool {
	if c.Country.Alfa2 != "US" {
		return false
	}

	if _, ok := usaWC[c.Region.ISOCode]; ok {
		return true
	}

	return false
}

func (c *City) IsUSAEastCost() bool {
	if c.Country.Alfa2 != "US" {
		return false
	}

	return !c.IsUSAWestCost()
}

var euCountries = map[string]struct{}{
	"AT": {},
	"BE": {},
	"HR": {},
	"BG": {},
	"CY": {},
	"CZ": {},
	"DK": {},
	"EE": {},
	"FI": {},
	"FR": {},
	"DE": {},
	"GR": {},
	"HU": {},
	"IE": {},
	"IT": {},
	"LV": {},
	"LT": {},
	"LU": {},
	"MT": {},
	"NL": {},
	"PL": {},
	"PT": {},
	"RO": {},
	"SK": {},
	"SI": {},
	"ES": {},
	"SE": {},
	"GB": {},
}

func (c *Country) IsGDPRZone() bool {
	return c.IsEU()
}

func (c *Country) IsGPPZone() bool {
	return c.IsEU() || c.Alfa2 == "US" || c.Alfa2 == "CA"
}

func (c *Country) IsEU() bool {
	if _, ok := euCountries[c.Alfa2]; ok {
		return true
	}

	return false
}
