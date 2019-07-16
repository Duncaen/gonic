package handler

import (
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"regexp"
	"strings"
	"testing"

	jd "github.com/josephburnett/jd/lib"

	"senan.xyz/g/gonic/db"
)

var (
	testDataDir    = "testdata"
	testCamelExpr  = regexp.MustCompile("([a-z0-9])([A-Z])")
	testDBPath     = path.Join(testDataDir, "db")
	testController *Controller
)

func init() {
	db, err := db.New(testDBPath)
	if err != nil {
		log.Fatalf("error opening database: %v\n", err)
	}
	testController = &Controller{
		DB: db,
	}
}

type queryCase struct {
	params     url.Values
	expectPath string
	listSet    bool
}

func runQueryCases(t *testing.T, handler http.HandlerFunc, cases []*queryCase) {
	for _, qc := range cases {
		qc := qc // pin
		t.Run(qc.expectPath, func(t *testing.T) {
			t.Parallel()
			//
			// ensure the handlers give us json
			qc.params.Add("f", "json")
			//
			// request from the handler in question
			req, _ := http.NewRequest("", "?"+qc.params.Encode(), nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			body := rr.Body.String()
			if status := rr.Code; status != http.StatusOK {
				t.Fatalf("didn't give a 200\n%s", body)
			}
			//
			// convert test name to query case path
			snake := testCamelExpr.ReplaceAllString(t.Name(), "${1}_${2}")
			lower := strings.ToLower(snake)
			relPath := strings.Replace(lower, "/", "_", -1)
			absExpPath := path.Join(testDataDir, relPath)
			//
			// read case to differ with handler result
			expected, err := jd.ReadJsonFile(absExpPath)
			if err != nil {
				t.Fatalf("parsing expected: %v", err)
			}
			actual, _ := jd.ReadJsonString(body)
			if err != nil {
				t.Fatalf("parsing actual: %v", err)
			}
			diffOpts := []jd.Metadata{}
			if qc.listSet {
				diffOpts = append(diffOpts, jd.SET)
			}
			diff := expected.Diff(actual, diffOpts...)
			//
			// pass or fail
			if len(diff) == 0 {
				return
			}
			t.Errorf("\u001b[31;1mdiffering json\u001b[0m\n%s",
				diff.Render())
		})
	}
}
