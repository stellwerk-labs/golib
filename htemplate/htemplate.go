package htemplate

import (
	ttemplate "text/template"

	"github.com/Masterminds/sprig/v3"
)

func TxtFuncMap() ttemplate.FuncMap {
	fm := make(map[string]interface{})
	gfm := sprig.GenericFuncMap()

	// Date functions
	fm["ago"] = gfm["ago"]
	fm["date"] = gfm["date"]
	fm["date_in_zone"] = gfm["date_in_zone"]
	fm["date_modify"] = gfm["date_modify"]
	fm["dateInZone"] = gfm["dateInZone"]
	fm["dateModify"] = gfm["dateModify"]
	fm["duration"] = gfm["duration"]
	fm["durationRound"] = gfm["durationRound"]
	fm["htmlDate"] = gfm["htmlDate"]
	fm["htmlDateInZone"] = gfm["htmlDateInZone"]
	fm["must_date_modify"] = gfm["must_date_modify"]
	fm["mustDateModify"] = gfm["mustDateModify"]
	fm["mustToDate"] = gfm["mustToDate"]
	fm["now"] = gfm["now"]
	fm["toDate"] = gfm["toDate"]
	fm["unixEpoch"] = gfm["unixEpoch"]

	// Strings
	fm["abbrev"] = gfm["abbrev"]
	fm["abbrevboth"] = gfm["abbrevboth"]
	fm["trunc"] = gfm["trunc"]
	fm["trim"] = gfm["trim"]
	fm["upper"] = gfm["upper"]
	fm["lower"] = gfm["lower"]
	fm["title"] = gfm["title"]
	fm["untitle"] = gfm["untitle"]
	fm["substr"] = gfm["substr"]
	fm["repeat"] = gfm["repeat"]
	fm["trimAll"] = gfm["trimAll"]
	fm["trimSuffix"] = gfm["trimSuffix"]
	fm["trimPrefix"] = gfm["trimPrefix"]
	fm["nospace"] = gfm["nospace"]
	fm["initials"] = gfm["initials"]
	fm["randAlphaNum"] = gfm["randAlphaNum"]
	fm["randAlpha"] = gfm["randAlpha"]
	fm["randAscii"] = gfm["randAscii"]
	fm["randNumeric"] = gfm["randNumeric"]
	fm["swapcase"] = gfm["swapcase"]
	fm["shuffle"] = gfm["shuffle"]
	fm["snakecase"] = gfm["snakecase"]
	fm["camelcase"] = gfm["camelcase"]
	fm["kebabcase"] = gfm["kebabcase"]
	fm["wrap"] = gfm["wrap"]
	fm["wrapWith"] = gfm["wrapWith"]
	fm["contains"] = gfm["contains"]
	fm["hasPrefix"] = gfm["hasPrefix"]
	fm["hasSuffix"] = gfm["hasSuffix"]
	fm["quote"] = gfm["quote"]
	fm["squote"] = gfm["squote"]
	fm["cat"] = gfm["cat"]
	fm["indent"] = gfm["indent"]
	fm["nindent"] = gfm["nindent"]
	fm["replace"] = gfm["replace"]
	fm["plural"] = gfm["plural"]
	fm["sha1sum"] = gfm["sha1sum"]
	fm["sha256sum"] = gfm["sha256sum"]
	fm["adler32sum"] = gfm["adler32sum"]
	fm["toString"] = gfm["toString"]

	fm["atoi"] = gfm["atoi"]
	fm["int64"] = gfm["int64"]
	fm["int"] = gfm["int"]
	fm["float64"] = gfm["float64"]
	fm["seq"] = gfm["seq"]
	fm["toDecimal"] = gfm["toDecimal"]

	// Split "/" foo/bar returns map[int]string{0: foo, 1: bar}
	fm["split"] = gfm["split"]
	fm["splitList"] = gfm["splitList"]
	// Splitn "/" foo/bar/fuu returns map[int]string{0: foo, 1: bar/fuu}
	fm["splitn"] = gfm["splitn"]
	fm["toStrings"] = gfm["toStrings"]

	fm["until"] = gfm["until"]
	fm["untilStep"] = gfm["untilStep"]

	// Basic arithmetic
	fm["add1"] = gfm["add1"]
	fm["add"] = gfm["add"]
	fm["sub"] = gfm["sub"]
	fm["div"] = gfm["div"]
	fm["mod"] = gfm["mod"]
	fm["mul"] = gfm["mul"]
	fm["randInt"] = gfm["randInt"]
	fm["add1f"] = gfm["add1f"]
	fm["addf"] = gfm["addf"]
	fm["subf"] = gfm["subf"]
	fm["divf"] = gfm["divf"]
	fm["mulf"] = gfm["mulf"]
	fm["biggest"] = gfm["biggest"]
	fm["max"] = gfm["max"]
	fm["min"] = gfm["min"]
	fm["maxf"] = gfm["maxf"]
	fm["minf"] = gfm["minf"]
	fm["ceil"] = gfm["ceil"]
	fm["floor"] = gfm["floor"]
	fm["round"] = gfm["round"]

	// string slices.
	fm["join"] = gfm["join"]
	fm["sortAlpha"] = gfm["sortAlpha"]

	// Defaults
	fm["default"] = gfm["default"]
	fm["empty"] = gfm["empty"]
	fm["coalesce"] = gfm["coalesce"]
	fm["all"] = gfm["all"]
	fm["any"] = gfm["any"]
	fm["compact"] = gfm["compact"]
	fm["mustCompact"] = gfm["mustCompact"]
	fm["fromJson"] = gfm["fromJson"]
	fm["toJson"] = gfm["toJson"]
	fm["toPrettyJson"] = gfm["toPrettyJson"]
	fm["toRawJson"] = gfm["toRawJson"]
	fm["mustFromJson"] = gfm["mustFromJson"]
	fm["mustToJson"] = gfm["mustToJson"]
	fm["mustToPrettyJson"] = gfm["mustToPrettyJson"]
	fm["mustToRawJson"] = gfm["mustToRawJson"]
	fm["ternary"] = gfm["ternary"]
	fm["deepCopy"] = gfm["deepCopy"]
	fm["mustDeepCopy"] = gfm["mustDeepCopy"]

	// Reflection
	fm["typeOf"] = gfm["typeOf"]
	fm["typeIs"] = gfm["typeIs"]
	fm["typeIsLike"] = gfm["typeIsLike"]
	fm["kindOf"] = gfm["kindOf"]
	fm["kindIs"] = gfm["kindIs"]
	fm["deepEqual"] = gfm["deepEqual"]

	// Paths
	fm["base"] = gfm["base"]
	fm["dir"] = gfm["dir"]
	fm["clean"] = gfm["clean"]
	fm["ext"] = gfm["ext"]
	fm["isAbs"] = gfm["isAbs"]

	// Filepaths
	fm["osBase"] = gfm["osBase"]
	fm["osClean"] = gfm["osClean"]
	fm["osDir"] = gfm["osDir"]
	fm["osExt"] = gfm["osExt"]
	fm["osIsAbs"] = gfm["osIsAbs"]

	// Encoding
	fm["b64enc"] = gfm["b64enc"]
	fm["b64dec"] = gfm["b64dec"]
	fm["b32enc"] = gfm["b32enc"]
	fm["b32dec"] = gfm["b32dec"]

	// Data Structures
	fm["tuple"] = gfm["tuple"]
	fm["list"] = gfm["list"]
	fm["dict"] = gfm["dict"]
	fm["get"] = gfm["get"]
	fm["set"] = gfm["set"]
	fm["unset"] = gfm["unset"]
	fm["hasKey"] = gfm["hasKey"]
	fm["pluck"] = gfm["pluck"]
	fm["keys"] = gfm["keys"]
	fm["pick"] = gfm["pick"]
	fm["omit"] = gfm["omit"]
	fm["merge"] = gfm["merge"]
	fm["mergeOverwrite"] = gfm["mergeOverwrite"]
	fm["mustMerge"] = gfm["mustMerge"]
	fm["mustMergeOverwrite"] = gfm["mustMergeOverwrite"]
	fm["values"] = gfm["values"]

	fm["append"] = gfm["append"]
	fm["mustAppend"] = gfm["mustAppend"]
	fm["prepend"] = gfm["prepend"]
	fm["mustPrepend"] = gfm["mustPrepend"]
	fm["first"] = gfm["first"]
	fm["mustFirst"] = gfm["mustFirst"]
	fm["rest"] = gfm["rest"]
	fm["mustRest"] = gfm["mustRest"]
	fm["last"] = gfm["last"]
	fm["mustLast"] = gfm["mustLast"]
	fm["initial"] = gfm["initial"]
	fm["mustInitial"] = gfm["mustInitial"]
	fm["reverse"] = gfm["reverse"]
	fm["mustReverse"] = gfm["mustReverse"]
	fm["uniq"] = gfm["uniq"]
	fm["mustUniq"] = gfm["mustUniq"]
	fm["without"] = gfm["without"]
	fm["mustWithout"] = gfm["mustWithout"]
	fm["has"] = gfm["has"]
	fm["mustHas"] = gfm["mustHas"]
	fm["slice"] = gfm["slice"]
	fm["mustSlice"] = gfm["mustSlice"]
	fm["concat"] = gfm["concat"]
	fm["dig"] = gfm["dig"]
	fm["chunk"] = gfm["chunk"]
	fm["mustChunk"] = gfm["mustChunk"]

	// Crypto
	fm["bcrypt"] = gfm["bcrypt"]
	fm["htpasswd"] = gfm["htpasswd"]
	fm["genPrivateKey"] = gfm["genPrivateKey"]
	fm["derivePassword"] = gfm["derivePassword"]
	fm["buildCustomCert"] = gfm["buildCustomCert"]
	fm["genCA"] = gfm["genCA"]
	fm["genCAWithKey"] = gfm["genCAWithKey"]
	fm["genSelfSignedCert"] = gfm["genSelfSignedCert"]
	fm["genSelfSignedCertWithKey"] = gfm["genSelfSignedCertWithKey"]
	fm["genSignedCert"] = gfm["genSignedCert"]
	fm["genSignedCertWithKey"] = gfm["genSignedCertWithKey"]
	fm["encryptAES"] = gfm["encryptAES"]
	fm["decryptAES"] = gfm["decryptAES"]
	fm["randBytes"] = gfm["randBytes"]

	// UUIDs
	fm["uuidv4"] = gfm["uuidv4"]

	// SemVer
	fm["semver"] = gfm["semver"]
	fm["semverCompare"] = gfm["semverCompare"]

	// Flow Control
	fm["fail"] = gfm["fail"]

	// Regex
	fm["regexMatch"] = gfm["regexMatch"]
	fm["mustRegexMatch"] = gfm["mustRegexMatch"]
	fm["regexFindAll"] = gfm["regexFindAll"]
	fm["mustRegexFindAll"] = gfm["mustRegexFindAll"]
	fm["regexFind"] = gfm["regexFind"]
	fm["mustRegexFind"] = gfm["mustRegexFind"]
	fm["regexReplaceAll"] = gfm["regexReplaceAll"]
	fm["mustRegexReplaceAll"] = gfm["mustRegexReplaceAll"]
	fm["regexReplaceAllLiteral"] = gfm["regexReplaceAllLiteral"]
	fm["mustRegexReplaceAllLiteral"] = gfm["mustRegexReplaceAllLiteral"]
	fm["regexSplit"] = gfm["regexSplit"]
	fm["mustRegexSplit"] = gfm["mustRegexSplit"]
	fm["regexQuoteMeta"] = gfm["regexQuoteMeta"]

	// URLs
	fm["urlParse"] = gfm["urlParse"]
	fm["urlJoin"] = gfm["urlJoin"]

	return ttemplate.FuncMap(fm)
}
