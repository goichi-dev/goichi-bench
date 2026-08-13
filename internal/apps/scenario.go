package apps

// Scenario is one request shape, issued identically to every framework.
type Scenario struct {
	Key    string
	Title  string
	Desc   string
	Method string
	Path   string
	Body   string
}

// IDResult is returned by the single-parameter routes.
type IDResult struct {
	ID string `json:"id"`
}

// Scenarios are the REST scenarios measured by both phases, in report order.
var Scenarios = []Scenario{
	{
		Key:    "static",
		Title:  "Static route",
		Desc:   "GET /ping — no parameters, plain-text body. Pure dispatch cost.",
		Method: "GET",
		Path:   "/ping",
	},
	{
		Key:    "param",
		Title:  "One path parameter",
		Desc:   "GET /user/:id — one parameter captured and echoed back as JSON.",
		Method: "GET",
		Path:   "/user/4242",
	},
	{
		Key:    "params3",
		Title:  "Three path parameters",
		Desc:   "GET /user/:id/posts/:pid/comments/:cid — deep tree walk, three captures.",
		Method: "GET",
		Path:   "/user/4242/posts/77/comments/9",
	},
	{
		Key:    "json",
		Title:  "JSON response",
		Desc:   "GET /json — serialise a fixed six-field struct.",
		Method: "GET",
		Path:   "/json",
	},
	{
		Key:    "post",
		Title:  "JSON request body",
		Desc:   "POST /echo — bind a JSON body with each framework's own binder, then write it back.",
		Method: "POST",
		Path:   "/echo",
		Body:   EchoBody,
	},
	{
		Key:    "many",
		Title:  "150-route table",
		Desc:   "GET a parameterised route registered among 150 others. Router scalability.",
		Method: "GET",
		Path:   "/api/v1/segment42/4242/detail",
	},
}

// ScenarioByKey returns a scenario by its key.
func ScenarioByKey(key string) (Scenario, bool) {
	for _, s := range Scenarios {
		if s.Key == key {
			return s, true
		}
	}
	return Scenario{}, false
}

// ExpectedBody is the response body every framework must produce for a
// scenario. The correctness gate compares against this before any timing is
// recorded, so a framework cannot look fast by doing less work.
func ExpectedBody(key string) string {
	switch key {
	case "static":
		return "pong"
	case "param", "many":
		return `{"id":"4242"}`
	case "params3":
		return `{"user":"4242","post":"77","comment":"9"}`
	case "json":
		return `{"id":1,"name":"Somchai Jaidee","email":"somchai@example.com","active":true,"balance":1234.56,"tags":["staff","verified","th"]}`
	case "post":
		return `{"name":"Somchai Jaidee","email":"somchai@example.com","age":37}`
	}
	return ""
}
