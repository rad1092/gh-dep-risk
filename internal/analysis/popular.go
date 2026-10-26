package analysis

import "strings"

// popularNPMPackages are widely used unscoped npm packages. A newly added
// package whose name is one edit away from one of these is a common sign of
// typosquatting, so it is flagged for a closer look. Names shorter than five
// characters are left out because one edit turns them into too many real
// packages.
var popularNPMPackages = []string{
	"accepts", "acorn", "adm-zip", "ajv", "angular", "ansi-regex", "ansi-styles", "antd", "anymatch",
	"apollo-client", "apollo-server", "archiver", "argparse", "async", "autoprefixer", "aws-sdk", "axios",
	"babel-core", "babel-eslint", "babel-loader", "babel-preset-env", "balanced-match", "bcrypt", "bcryptjs",
	"big.js", "bignumber.js", "bindings", "bluebird", "body-parser", "bootstrap", "boxen", "brace-expansion",
	"braces", "browserify", "browserslist", "buffer", "bunyan", "caniuse-lite", "canvas", "chalk", "chart.js",
	"cheerio", "chokidar", "class-validator", "classnames", "clean-css", "cli-table", "clone", "coffee-script",
	"color", "colord", "colors", "commander", "compression", "concat-stream", "concurrently", "config", "connect",
	"cookie", "cookie-parser", "cookies", "core-js", "cross-env", "cross-fetch", "cross-spawn", "crypto-js", "css-loader",
	"csv-parse", "cypress", "d3-scale", "date-fns", "dayjs", "debug", "decimal.js", "deepmerge", "del-cli",
	"discord.js", "dotenv", "dotenv-expand", "electron", "electron-builder", "electron-updater", "enzyme",
	"es6-promise", "esbuild", "escape-string-regexp", "eslint", "eslint-config-airbnb", "eslint-config-prettier",
	"eslint-plugin-import", "eslint-plugin-react", "esprima", "estraverse", "ethers", "event-stream", "eventemitter3",
	"execa", "express", "express-session", "extend", "fast-csv", "fast-glob", "file-loader", "fill-range", "firebase",
	"follow-redirects", "form-data", "fs-extra", "fsevents", "gatsby", "graceful-fs", "graphql", "gulp", "grunt",
	"handlebars", "has-flag", "helmet", "highlight.js", "html-webpack-plugin", "http-proxy", "http-server",
	"https-proxy-agent", "husky", "iconv-lite", "immer", "immutable", "inflight", "inherits", "inquirer", "ioredis",
	"is-number", "isomorphic-fetch", "jasmine", "jquery", "js-yaml", "jsdoc", "jsdom", "jsonwebtoken", "jszip", "karma",
	"kind-of", "lint-staged", "lodash", "lodash.merge", "log4js", "lru-cache", "luxon", "markdown-it", "marked",
	"material-ui", "micromatch", "mime-types", "minimatch", "minimist", "mkdirp", "mocha", "moment",
	"moment-timezone", "mongodb", "mongoose", "morgan", "multer", "mustache", "mysql", "mysql2", "nanoid",
	"next-auth", "node-fetch", "node-gyp", "node-sass", "nodemailer", "nodemon", "nuxt", "numeral", "nunjucks",
	"object-assign", "openai", "papaparse", "parcel", "passport", "passport-jwt", "path-to-regexp", "pdfkit",
	"picocolors", "picomatch", "pinia", "playwright", "postcss", "postcss-loader", "preact", "prettier", "prisma",
	"prop-types", "puppeteer", "puppeteer-core", "query-string", "ramda", "react", "react-dom", "react-native",
	"react-redux", "react-router", "react-router-dom", "react-scripts", "readable-stream", "readdirp", "redis",
	"redux", "redux-thunk", "regenerator-runtime", "request", "request-promise", "rimraf", "rollup", "rxjs",
	"safe-buffer", "sass-loader", "selenium-webdriver", "semver", "sequelize", "serve-static", "sharp", "shelljs",
	"shortid", "signal-exit", "sinon", "socket.io", "socket.io-client", "solid-js", "source-map",
	"source-map-support", "sqlite3", "string-width", "strip-ansi", "stripe", "style-loader", "styled-components",
	"superagent", "supertest", "supports-color", "svelte", "tailwindcss", "terser", "three", "through2", "tslib",
	"tslint", "ts-jest", "ts-node", "twilio", "typeorm", "typescript", "uglify-js", "underscore", "url-loader", "uuid",
	"validator", "vitest", "vue-router", "vuex", "web3", "webdriverio", "webpack", "webpack-cli",
	"webpack-dev-server", "whatwg-fetch", "winston", "xlsx", "xml2js", "yargs", "yarn",
}

var popularNPMSet = func() map[string]struct{} {
	set := make(map[string]struct{}, len(popularNPMPackages))
	for _, name := range popularNPMPackages {
		set[name] = struct{}{}
	}
	return set
}()

// SimilarPopularPackage returns the popular package that name is one edit
// (an insertion, deletion, substitution, or swap of neighbouring characters)
// away from, or "" when there is none. Scoped names and popular names
// themselves never match.
func SimilarPopularPackage(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if strings.HasPrefix(name, "@") || len(name) < 5 {
		return ""
	}
	if _, ok := popularNPMSet[name]; ok {
		return ""
	}
	for _, popular := range popularNPMPackages {
		if len(popular) >= 5 && withinOneEdit(name, popular) {
			return popular
		}
	}
	return ""
}

func withinOneEdit(a, b string) bool {
	if a == b {
		return false
	}
	switch len(a) - len(b) {
	case 0:
		first := -1
		for i := 0; i < len(a); i++ {
			if a[i] == b[i] {
				continue
			}
			if first >= 0 {
				// Two differences are one edit only when they are a swap of
				// neighbouring characters.
				return i == first+1 && a[first] == b[i] && a[i] == b[first] && a[i+1:] == b[i+1:]
			}
			first = i
		}
		return true
	case 1:
		return oneDeletion(a, b)
	case -1:
		return oneDeletion(b, a)
	default:
		return false
	}
}

// oneDeletion reports whether deleting one character from longer gives shorter.
func oneDeletion(longer, shorter string) bool {
	for i := 0; i < len(shorter); i++ {
		if longer[i] != shorter[i] {
			return longer[i+1:] == shorter[i:]
		}
	}
	return true
}
