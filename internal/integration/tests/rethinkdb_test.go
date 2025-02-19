package tests

import (
	"flag"
	"fmt"
	"github.com/segmentio/encoding/json"
	"math/rand"
	"os"
	"runtime"
	"testing"
	"time"

	test "gopkg.in/check.v1"
	r "gopkg.in/rethinkdb/rethinkdb-go.v6"
)

var session *r.Session
var testdata = flag.Bool("rethinkdb.testdata", true, "create test data")
var url, url1, url2, url3, db, authKey string

func init() {
	// Fixing test.testlogfile parsing error on Go 1.13+.
	if runtime.Version() < "go1.13" {
		flag.Parse()
	}

	r.SetVerbose(true)

	// If the test is being run by wercker look for the rethink url
	url = os.Getenv("RETHINKDB_URL")
	if url == "" {
		url = "localhost:28015"
	}

	url1 = os.Getenv("RETHINKDB_URL_1")
	if url1 == "" {
		url1 = "localhost:28016"
	}

	url2 = os.Getenv("RETHINKDB_URL_2")
	if url2 == "" {
		url2 = "localhost:28017"
	}

	url3 = os.Getenv("RETHINKDB_URL_3")
	if url3 == "" {
		url3 = "localhost:28018"
	}

	db = os.Getenv("RETHINKDB_DB")
	if db == "" {
		db = "test"
	}
}

// Begin TestMain(), Setup, Teardown
func testSetup(m *testing.M) {
	var err error
	session, err = r.Connect(r.ConnectOpts{
		Address: url,
	})
	if err != nil {
		r.Log.Fatalln(err.Error())
	}

	setupTestData()
}
func testTeardown(m *testing.M) {
	session.Close()
}

func testBenchmarkSetup() {
	r.DBDrop("benchmarks").Exec(session)
	r.DBCreate("benchmarks").Exec(session)

	r.DB("benchmarks").TableDrop("benchmarks").Run(session)
	r.DB("benchmarks").TableCreate("benchmarks").Run(session)
}

func testBenchmarkTeardown() {
	r.DBDrop("benchmarks").Run(session)
}

func TestMain(m *testing.M) {
	// seed randomness for use with tests
	rand.Seed(time.Now().UTC().UnixNano())

	testSetup(m)
	testBenchmarkSetup()
	res := m.Run()
	testBenchmarkTeardown()
	testTeardown(m)

	os.Exit(res)
}

//
// End TestMain(), Setup, Teardown
//

// Hook up gocheck into the gotest runner.
func Test(t *testing.T) { test.TestingT(t) }

type RethinkSuite struct{}

var _ = test.Suite(&RethinkSuite{})

// Expressions used in tests
var now = time.Now()
var arr = []interface{}{1, 2, 3, 4, 5, 6, 7, 8, 9}
var darr = []interface{}{1, 1, 2, 2, 3, 3, 5, 5, 6}
var narr = []interface{}{
	1, 2, 3, 4, 5, 6, []interface{}{
		7.1, 7.2, 7.3,
	},
}
var obj = map[string]interface{}{"a": 1, "b": 2, "c": 3}
var nobj = map[string]interface{}{
	"A": 1,
	"B": 2,
	"C": map[string]interface{}{
		"1": 3,
		"2": 4,
	},
}

var noDupNumObjList = []interface{}{
	map[string]interface{}{"id": 1, "g1": 1, "g2": 1, "num": 0},
	map[string]interface{}{"id": 2, "g1": 2, "g2": 2, "num": 5},
	map[string]interface{}{"id": 3, "g1": 3, "g2": 2, "num": 10},
	map[string]interface{}{"id": 5, "g1": 2, "g2": 3, "num": 100},
	map[string]interface{}{"id": 6, "g1": 1, "g2": 1, "num": 15},
	map[string]interface{}{"id": 8, "g1": 4, "g2": 2, "num": 50},
	map[string]interface{}{"id": 9, "g1": 2, "g2": 3, "num": 25},
}
var objList = []interface{}{
	map[string]interface{}{"id": 1, "g1": 1, "g2": 1, "num": 0},
	map[string]interface{}{"id": 2, "g1": 2, "g2": 2, "num": 5},
	map[string]interface{}{"id": 3, "g1": 3, "g2": 2, "num": 10},
	map[string]interface{}{"id": 4, "g1": 2, "g2": 3, "num": 0},
	map[string]interface{}{"id": 5, "g1": 2, "g2": 3, "num": 100},
	map[string]interface{}{"id": 6, "g1": 1, "g2": 1, "num": 15},
	map[string]interface{}{"id": 7, "g1": 1, "g2": 2, "num": 0},
	map[string]interface{}{"id": 8, "g1": 4, "g2": 2, "num": 50},
	map[string]interface{}{"id": 9, "g1": 2, "g2": 3, "num": 25},
}
var nameList = []interface{}{
	map[string]interface{}{"id": 1, "first_name": "John", "last_name": "Smith", "gender": "M"},
	map[string]interface{}{"id": 2, "first_name": "Jane", "last_name": "Smith", "gender": "F"},
}

type TStr string
type TMap map[string]interface{}

type T struct {
	A string `rethinkdb:"id, omitempty"`
	B int
	C int `rethinkdb:"-"`
	D map[string]interface{}
	E []interface{}
	F X
}

type SimpleT struct {
	A string
	B int
}

type X struct {
	XA int
	XB string
	XC []string
	XD Y
	XE TStr
	XF []TStr
}

type Y struct {
	YA int
	YB map[string]interface{}
	YC map[string]string
	YD TMap
}

type PseudoTypes struct {
	T time.Time
	Z time.Time
	B []byte
}

var str = T{
	A: "A",
	B: 1,
	C: 1,
	D: map[string]interface{}{
		"D1": 1,
		"D2": "2",
	},
	E: []interface{}{
		"E1", "E2", "E3", 4,
	},
	F: X{
		XA: 2,
		XB: "B",
		XC: []string{"XC1", "XC2"},
		XD: Y{
			YA: 3,
			YB: map[string]interface{}{
				"1": "1",
				"2": "2",
				"3": 3,
			},
			YC: map[string]string{
				"YC1": "YC1",
			},
			YD: TMap{
				"YD1": "YD1",
			},
		},
		XE: "XE",
		XF: []TStr{
			"XE1", "XE2",
		},
	},
}

type Author struct {
	ID   string `rethinkdb:"id,omitempty"`
	Name string `rethinkdb:"name"`
}

type Book struct {
	ID     string `rethinkdb:"id,omitempty"`
	Title  string `rethinkdb:"title"`
	Author Author `rethinkdb:"author_id,reference" rethinkdb_ref:"id"`
}

type TagsTest struct {
	A string `rethinkdb:"a"`
	B string `json:"b"`
	C string `rethinkdb:"c1" json:"c2"`
}

func (s *RethinkSuite) BenchmarkExpr(c *test.C) {
	for i := 0; i < c.N; i++ {
		// Test query
		query := r.Expr(true)
		err := query.Exec(session)
		c.Assert(err, test.IsNil)
	}
}

func (s *RethinkSuite) BenchmarkNoReplyExpr(c *test.C) {
	for i := 0; i < c.N; i++ {
		// Test query
		query := r.Expr(true)
		err := query.Exec(session, r.ExecOpts{NoReply: true})
		c.Assert(err, test.IsNil)
	}
}

func (s *RethinkSuite) BenchmarkGet(c *test.C) {
	// Ensure table + database exist
	r.DBCreate("testb1").RunWrite(session)
	r.DB("testb1").TableCreate("TestManyBench1").RunWrite(session)
	r.DB("testb1").Table("TestManyBench1").Delete().RunWrite(session)

	// Insert rows
	data := []interface{}{}
	for i := 0; i < 100; i++ {
		data = append(data, map[string]interface{}{
			"id": i,
		})
	}
	r.DB("testb1").Table("TestManyBench1").Insert(data).Run(session)

	for i := 0; i < c.N; i++ {
		n := rand.Intn(100)

		// Test query
		var response interface{}
		query := r.DB("testb1").Table("TestManyBench1").Get(n)
		res, err := query.Run(session)
		c.Assert(err, test.IsNil)

		err = res.One(&response)

		c.Assert(err, test.IsNil)
		c.Assert(response, JsonEquals, map[string]interface{}{"id": n})
	}
}

func (s *RethinkSuite) BenchmarkGetStruct(c *test.C) {
	// Ensure table + database exist
	r.DBCreate("testb2").RunWrite(session)
	r.DB("testb2").TableCreate("TestManyBench2").RunWrite(session)
	r.DB("testb2").Table("TestManyBench2").Delete().RunWrite(session)

	// Insert rows
	data := []interface{}{}
	for i := 0; i < 100; i++ {
		data = append(data, map[string]interface{}{
			"id":   i,
			"name": "Object 1",
			"Attrs": []interface{}{map[string]interface{}{
				"Name":  "attr 1",
				"Value": "value 1",
			}},
		})
	}
	r.DB("testb2").Table("TestManyBench2").Insert(data).Run(session)

	for i := 0; i < c.N; i++ {
		n := rand.Intn(100)

		// Test query
		var resObj object
		query := r.DB("testb2").Table("TestManyBench2").Get(n)
		res, err := query.Run(session)
		c.Assert(err, test.IsNil)

		err = res.One(&resObj)

		c.Assert(err, test.IsNil)
	}
}

func (s *RethinkSuite) BenchmarkSelectMany(c *test.C) {
	// Ensure table + database exist
	r.DBCreate("testb3").RunWrite(session)
	r.DB("testb3").TableCreate("TestManyBench3").RunWrite(session)
	r.DB("testb3").Table("TestManyBench3").Delete().RunWrite(session)

	// Insert rows
	data := []interface{}{}
	for i := 0; i < 100; i++ {
		data = append(data, map[string]interface{}{
			"id": i,
		})
	}
	r.DB("testb3").Table("TestManyBench3").Insert(data).Run(session)

	for i := 0; i < c.N; i++ {
		// Test query
		res, err := r.DB("testb3").Table("TestManyBench3").Run(session)
		c.Assert(err, test.IsNil)

		var response []map[string]interface{}
		err = res.All(&response)

		c.Assert(err, test.IsNil)
		c.Assert(response, test.HasLen, 100)
	}
}

func (s *RethinkSuite) BenchmarkSelectManyStruct(c *test.C) {
	// Ensure table + database exist
	r.DBCreate("testb4").RunWrite(session)
	r.DB("testb4").TableCreate("TestManyBench4").RunWrite(session)
	r.DB("testb4").Table("TestManyBench4").Delete().RunWrite(session)

	// Insert rows
	data := []interface{}{}
	for i := 0; i < 100; i++ {
		data = append(data, map[string]interface{}{
			"id":   i,
			"name": "Object 1",
			"Attrs": []interface{}{map[string]interface{}{
				"Name":  "attr 1",
				"Value": "value 1",
			}},
		})
	}
	r.DB("testb4").Table("TestManyBench4").Insert(data).Run(session)

	for i := 0; i < c.N; i++ {
		// Test query
		res, err := r.DB("testb4").Table("TestManyBench4").Run(session)
		c.Assert(err, test.IsNil)

		var response []object
		err = res.All(&response)

		c.Assert(err, test.IsNil)
		c.Assert(response, test.HasLen, 100)
	}
}

// Test utils

// Print variable as JSON
func jsonPrint(v interface{}) {
	b, err := json.MarshalIndent(v, "", "    ")
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(string(b))
}
