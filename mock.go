package rethinkdb

import (
	"encoding/binary"
	"fmt"
	"github.com/segmentio/encoding/json"
	"gopkg.in/rethinkdb/rethinkdb-go.v6/encoding"
	"net"
	"reflect"
	"sync"
	"time"

	"golang.org/x/net/context"
	p "gopkg.in/rethinkdb/rethinkdb-go.v6/ql2"
)

// Mocking is based on the amazing package github.com/stretchr/testify

// testingT is an interface wrapper around *testing.T
type testingT interface {
	Logf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	FailNow()
}

// MockAnything can be used in place of any term, this is useful when you want
// mock similar queries or queries that you don't quite know the exact structure
// of.
func MockAnything() Term {
	t := constructRootTerm("MockAnything", p.Term_DATUM, nil, nil)
	t.isMockAnything = true

	return t
}

func (t Term) MockAnything() Term {
	t = constructMethodTerm(t, "MockAnything", p.Term_DATUM, nil, nil)
	t.isMockAnything = true

	return t
}

// MockQuery represents a mocked query and is used for setting expectations,
// as well as recording activity.
type MockQuery struct {
	parent *Mock

	// Holds the query and term
	Query Query

	// Holds the JSON representation of query
	BuiltQuery []byte

	// Holds the response that should be returned when this method is executed.
	Response interface{}

	// Holds the error that should be returned when this method is executed.
	Error error

	// The number of times to return the return arguments when setting
	// expectations. 0 means to always return the value.
	Repeatability int

	// Holds a channel that will be used to block the Return until it either
	// recieves a message or is connClosed. nil means it returns immediately.
	WaitFor <-chan time.Time

	// Amount of times this query has been executed
	executed int
}

func newMockQuery(parent *Mock, q Query) *MockQuery {
	// Build and marshal term
	builtQuery, err := json.Marshal(q.Build())
	if err != nil {
		panic(fmt.Sprintf("Failed to build query: %s", err))
	}

	return &MockQuery{
		parent:        parent,
		Query:         q,
		BuiltQuery:    builtQuery,
		Response:      make([]interface{}, 0),
		Repeatability: 0,
		WaitFor:       nil,
	}
}

func newMockQueryFromTerm(parent *Mock, t Term, opts map[string]interface{}) *MockQuery {
	q, err := parent.newQuery(t, opts)
	if err != nil {
		panic(fmt.Sprintf("Failed to build query: %s", err))
	}

	return newMockQuery(parent, q)
}

func (mq *MockQuery) lock() {
	mq.parent.mu.Lock()
}

func (mq *MockQuery) unlock() {
	mq.parent.mu.Unlock()
}

// Return specifies the return arguments for the expectation.
//
//	mock.On(r.Table("test")).Return(nil, errors.New("failed"))
//
// values of `chan []interface{}` type will turn to delayed data that produce data
// when there is an elements available on the channel. These elements are chunk of responses.
// Values of `func() []interface{}` type will produce data by calling the function. E.g.
// Closing channel or returning nil from func means end of data.
//
//	f := func() []interface{} { return []interface{}{1, 2} }
//	mock.On(r.Table("test1")).Return(f)
//
//	ch := make(chan []interface{})
//	mock.On(r.Table("test1")).Return(ch)
//
//	Running the query above will block until a value is pushed onto ch.
func (mq *MockQuery) Return(response interface{}, err error) *MockQuery {
	mq.lock()
	defer mq.unlock()

	mq.Response = response
	mq.Error = err

	return mq
}

// Once indicates that that the mock should only return the value once.
//
//	mock.On(r.Table("test")).Return(result, nil).Once()
func (mq *MockQuery) Once() *MockQuery {
	return mq.Times(1)
}

// Twice indicates that that the mock should only return the value twice.
//
//	mock.On(r.Table("test")).Return(result, nil).Twice()
func (mq *MockQuery) Twice() *MockQuery {
	return mq.Times(2)
}

// Times indicates that that the mock should only return the indicated number
// of times.
//
//	mock.On(r.Table("test")).Return(result, nil).Times(5)
func (mq *MockQuery) Times(i int) *MockQuery {
	mq.lock()
	defer mq.unlock()
	mq.Repeatability = i
	return mq
}

// WaitUntil sets the channel that will block the mock's return until its connClosed
// or a message is received.
//
//	mock.On(r.Table("test")).WaitUntil(time.After(time.Second))
func (mq *MockQuery) WaitUntil(w <-chan time.Time) *MockQuery {
	mq.lock()
	defer mq.unlock()
	mq.WaitFor = w
	return mq
}

// After sets how long to block until the query returns
//
//	mock.On(r.Table("test")).After(time.Second)
func (mq *MockQuery) After(d time.Duration) *MockQuery {
	return mq.WaitUntil(time.After(d))
}

// On chains a new expectation description onto the mocked interface. This
// allows syntax like.
//
//	Mock.
//	   On(r.Table("test")).Return(result, nil).
//	   On(r.Table("test2")).Return(nil, errors.New("Some Error"))
func (mq *MockQuery) On(t Term) *MockQuery {
	return mq.parent.On(t)
}

// Mock is used to mock query execution and verify that the expected queries are
// being executed. Mocks are used by creating an instance using NewMock and then
// passing this when running your queries instead of a session. For example:
//
//	mock := r.NewMock()
//	mock.On(r.Table("test")).Return([]interface{}{data}, nil)
//
//	cursor, err := r.Table("test").Run(mock)
//
//	mock.AssertExpectations(t)
type Mock struct {
	mu   sync.Mutex
	opts ConnectOpts

	ExpectedQueries []*MockQuery
	Queries         []MockQuery
}

// NewMock creates an instance of Mock, you can optionally pass ConnectOpts to
// the function, if passed any mocked query will be generated using those
// options.
func NewMock(opts ...ConnectOpts) *Mock {
	m := &Mock{
		ExpectedQueries: make([]*MockQuery, 0),
		Queries:         make([]MockQuery, 0),
	}

	if len(opts) > 0 {
		m.opts = opts[0]
	}

	return m
}

// On starts a description of an expectation of the specified query
// being executed.
//
//	mock.On(r.Table("test"))
func (m *Mock) On(t Term, opts ...map[string]interface{}) *MockQuery {
	var qopts map[string]interface{}
	if len(opts) > 0 {
		qopts = opts[0]
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	mq := newMockQueryFromTerm(m, t, qopts)
	m.ExpectedQueries = append(m.ExpectedQueries, mq)
	return mq
}

// AssertExpectations asserts that everything specified with On and Return was
// in fact executed as expected. Queries may have been executed in any order.
func (m *Mock) AssertExpectations(t testingT) bool {
	var somethingMissing bool
	var failedExpectations int

	// iterate through each expectation
	expectedQueries := m.expectedQueries()
	for _, expectedQuery := range expectedQueries {
		if !m.queryWasExecuted(expectedQuery) && expectedQuery.executed == 0 {
			somethingMissing = true
			failedExpectations++
			t.Logf("❌\t%s", expectedQuery.Query.Term.String())
		} else {
			m.mu.Lock()
			if expectedQuery.Repeatability > 0 {
				somethingMissing = true
				failedExpectations++
			} else {
				t.Logf("✅\t%s", expectedQuery.Query.Term.String())
			}
			m.mu.Unlock()
		}
	}

	if somethingMissing {
		t.Errorf("FAIL: %d out of %d expectation(s) were met.\n\tThe query you are testing needs to be executed %d more times(s).", len(expectedQueries)-failedExpectations, len(expectedQueries), failedExpectations)
	}

	return !somethingMissing
}

// AssertNumberOfExecutions asserts that the query was executed expectedExecutions times.
func (m *Mock) AssertNumberOfExecutions(t testingT, expectedQuery *MockQuery, expectedExecutions int) bool {
	var actualExecutions int
	for _, query := range m.queries() {
		if query.Query.Term.compare(*expectedQuery.Query.Term, map[int64]int64{}) && query.Repeatability > -1 {
			// if bytes.Equal(query.BuiltQuery, expectedQuery.BuiltQuery) {
			actualExecutions++
		}
	}

	if expectedExecutions != actualExecutions {
		t.Errorf("Expected number of executions (%d) does not match the actual number of executions (%d).", expectedExecutions, actualExecutions)
		return false
	}

	return true
}

// AssertExecuted asserts that the method was executed.
// It can produce a false result when an argument is a pointer type and the underlying value changed after executing the mocked method.
func (m *Mock) AssertExecuted(t testingT, expectedQuery *MockQuery) bool {
	if !m.queryWasExecuted(expectedQuery) {
		t.Errorf("The query \"%s\" should have been executed, but was not.", expectedQuery.Query.Term.String())
		return false
	}
	return true
}

// AssertNotExecuted asserts that the method was not executed.
// It can produce a false result when an argument is a pointer type and the underlying value changed after executing the mocked method.
func (m *Mock) AssertNotExecuted(t testingT, expectedQuery *MockQuery) bool {
	if m.queryWasExecuted(expectedQuery) {
		t.Errorf("The query \"%s\" was executed, but should NOT have been.", expectedQuery.Query.Term.String())
		return false
	}
	return true
}

func (m *Mock) IsConnected() bool {
	return true
}

func (m *Mock) Query(ctx context.Context, q Query) (*Cursor, error) {
	found, query := m.findExpectedQuery(q)

	if found < 0 {
		panic(fmt.Sprintf("rethinkdb: mock: This query was unexpected:\n\t\t%s", q.Term.String()))
	} else {
		m.mu.Lock()
		switch {
		case query.Repeatability == 1:
			query.Repeatability = -1
			query.executed++

		case query.Repeatability > 1:
			query.Repeatability--
			query.executed++

		case query.Repeatability == 0:
			query.executed++
		}
		m.mu.Unlock()
	}

	// add the query
	m.mu.Lock()
	m.Queries = append(m.Queries, *newMockQuery(m, q))
	m.mu.Unlock()

	// block if specified
	if query.WaitFor != nil {
		<-query.WaitFor
	}

	// Return error without building cursor if non-nil
	if query.Error != nil {
		return nil, query.Error
	}

	if ctx == nil {
		ctx = context.Background()
	}

	conn := newConnection(newMockConn(query.Response), "mock", &ConnectOpts{})

	query.Query.Type = p.Query_CONTINUE
	query.Query.Token = conn.nextToken()

	// Build cursor and return
	c := newCursor(ctx, conn, "", query.Query.Token, query.Query.Term, query.Query.Opts)
	c.finished = true
	c.fetching = false
	c.isAtom = true
	c.finished = false
	c.releaseConn = func() error { return conn.Close() }

	conn.cursors[query.Query.Token] = c
	go conn.readSocket()
	go conn.processResponses()

	c.mu.Lock()
	err := c.fetchMore()
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (m *Mock) Exec(ctx context.Context, q Query) error {
	_, err := m.Query(ctx, q)

	return err
}

func (m *Mock) newQuery(t Term, opts map[string]interface{}) (Query, error) {
	return newQuery(t, opts, &m.opts)
}

func (m *Mock) findExpectedQuery(q Query) (int, *MockQuery) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, query := range m.ExpectedQueries {
		// if bytes.Equal(query.BuiltQuery, builtQuery) && query.Repeatability > -1 {
		if query.Query.Term.compare(*q.Term, map[int64]int64{}) && query.Repeatability > -1 {
			return i, query
		}
	}

	return -1, nil
}

func (m *Mock) queryWasExecuted(expectedQuery *MockQuery) bool {
	for _, query := range m.queries() {
		if query.Query.Term.compare(*expectedQuery.Query.Term, map[int64]int64{}) {
			// if bytes.Equal(query.BuiltQuery, expectedQuery.BuiltQuery) {
			return true
		}
	}

	// we didn't find the expected query
	return false
}

func (m *Mock) expectedQueries() []*MockQuery {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]*MockQuery{}, m.ExpectedQueries...)
}

func (m *Mock) queries() []MockQuery {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]MockQuery{}, m.Queries...)
}

type mockConn struct {
	mu          sync.Mutex
	value       []byte
	tokens      chan int64
	valueGetter func() []interface{}
}

func newMockConn(response interface{}) *mockConn {
	c := &mockConn{tokens: make(chan int64, 1)}
	switch g := response.(type) {
	case chan []interface{}:
		c.valueGetter = func() []interface{} { return <-g }
	case func() []interface{}:
		c.valueGetter = g
	default:
		responseVal := reflect.ValueOf(response)
		if responseVal.Kind() == reflect.Slice || responseVal.Kind() == reflect.Array {
			responses := make([]interface{}, responseVal.Len())
			for i := 0; i < responseVal.Len(); i++ {
				responses[i] = responseVal.Index(i).Interface()
			}
			c.valueGetter = funcGetter(responses)
		} else {
			c.valueGetter = funcGetter([]interface{}{response})
		}
	}
	return c
}

func funcGetter(responses []interface{}) func() []interface{} {
	done := false
	return func() []interface{} {
		if done {
			return nil
		}
		done = true
		return responses
	}
}

func (c *mockConn) Read(b []byte) (n int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.value == nil {
		values := c.valueGetter()

		jresps := make([]json.RawMessage, len(values))
		for i := range values {
			coded, err := encoding.Encode(values[i])
			if err != nil {
				panic(fmt.Sprintf("failed to encode response: %v", err))
			}
			jresps[i], err = json.Marshal(coded)
			if err != nil {
				panic(fmt.Sprintf("failed to encode response: %v", err))
			}
		}

		token := <-c.tokens
		resp := Response{
			Token:     token,
			Responses: jresps,
			Type:      p.Response_SUCCESS_PARTIAL,
		}
		if values == nil {
			resp.Type = p.Response_SUCCESS_SEQUENCE
		}

		c.value, err = json.Marshal(resp)
		if err != nil {
			panic(fmt.Sprintf("failed to encode response: %v", err))
		}

		if len(b) != respHeaderLen {
			panic("wrong header len")
		}
		binary.LittleEndian.PutUint64(b[:8], uint64(token))
		binary.LittleEndian.PutUint32(b[8:], uint32(len(c.value)))
		return len(b), nil
	} else {
		copy(b, c.value)
		c.value = nil
		return len(b), nil
	}
}

func (c *mockConn) Write(b []byte) (n int, err error) {
	if len(b) < 8 {
		panic("connBad socket write")
	}
	token := int64(binary.LittleEndian.Uint64(b[:8]))
	c.tokens <- token
	return len(b), nil
}
func (c *mockConn) Close() error                       { return nil }
func (c *mockConn) LocalAddr() net.Addr                { panic("not implemented") }
func (c *mockConn) RemoteAddr() net.Addr               { panic("not implemented") }
func (c *mockConn) SetDeadline(t time.Time) error      { panic("not implemented") }
func (c *mockConn) SetReadDeadline(t time.Time) error  { panic("not implemented") }
func (c *mockConn) SetWriteDeadline(t time.Time) error { panic("not implemented") }
