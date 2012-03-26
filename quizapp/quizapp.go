package myapp

import (
	"appengine"
	"appengine/datastore"
	"appengine/user"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"text/template"
	"time"
)

type Page struct {
	Title   string
	Content string
}

type Question struct {
	Text          string
	AnswerOptions []string // for multiple choice type questions
	Answer        string
	AnswerType    string // "text", "int", "float", ...
	IsStop        bool   // Must they answer this question before going on?
}

type Quiz struct {
	Title       string
	ID          string
	Created     time.Time
	OwnerID     string `json:"-"` // ownerID not sent via JSON
	QuestionsM  string `json:"-"`// marshaled into JSON for storage in the datastore
	Questions   []Question `datastore:"-"` // sent via the network
}

type JSONError struct {
	Name string `json:"name"`
	Code int `json:"code"`
	Message string `json:"message"`
	Error interface{} `json:"error"`
}

var (
	pageTemplate     = template.Must(template.ParseFiles("html/page.html", "html/boring.html"))
	adminTemplate    = template.Must(template.ParseFiles("html/page.html", "html/admin.html"))
	instanceHitCount = 0

	ErrorField = "error"
	// Error codes to use in the JSON response
	ErrorAuth = JSONError{"AUTH", 401, "Authentication required", nil}
	ErrorDatastore = JSONError{"DATA", 510, "Internal Datastore Error", nil}
	ErrorFormat = JSONError{"FORMAT", 400, "Bad request", nil }
	ErrorOther = JSONError{"OTHER", 500, "Other error", nil}

	valid_atype = map[string]bool { "text" : true, "mc": true, "int": true, "float": true, "duration": true }
)

func genQuizID() string {
	b := make([]byte, 16)
	if n, err := rand.Read(b); n != 16 {
		panic(fmt.Sprintf("could not read 16 random bytes.  I'm very unhappy: %s", err))
	}
	return base32.StdEncoding.EncodeToString(b)[0:24]
}

func init() {
	http.HandleFunc("/admin", adminHandler)
	http.HandleFunc("/q", queryHandler)
	http.Handle("/ql", AuthHandlerFunc(quizListHandler))
	http.Handle("/qc", AuthHandlerFunc(quizCreateHandler))
	http.Handle("/qget", AuthHandlerFunc(quizGetHandler))
	http.Handle("/qu", AuthHandlerFunc(quizUpdateHandler))
}

type AuthHandlerFunc func(http.ResponseWriter, *http.Request, appengine.Context, *user.User, map[string]interface{})

func (f AuthHandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	jsonAuthHandler(w, r, f)
}

func jsonAuthHandler(w http.ResponseWriter, r *http.Request, handler func(http.ResponseWriter, *http.Request, appengine.Context, *user.User, map[string]interface{})) {
	resp := make(map[string]interface{})
	c := appengine.NewContext(r)
	if u := user.Current(c); u == nil {
		c.Infof("jsonAuthHandler punting:  no valid user for %v", r.URL)
		resp[ErrorField] = ErrorAuth
	} else {
		handler(w, r, c, u, resp)
	}
	w.Header().Set("Content-Type", "text/javascript")
	b, _ := json.Marshal(resp)
	w.Write(b)
	return
}

func quizUpdateHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, u *user.User, resp map[string]interface{}) {
	var q Quiz
	var nq Quiz
	if err := json.Unmarshal([]byte(r.FormValue("q")), &nq); err != nil {
		c.Infof("Unmarshal json failed on %v", r.FormValue("q"))
		resp[ErrorField] = ErrorOther
		return
	}
	quizID := nq.ID
	k := datastore.NewKey(c, "Quiz", quizID, 0, nil)
	// sanity check quizID, please
	if err := datastore.Get(c, k, &q); err != nil {
		resp[ErrorField] = ErrorDatastore
		return
	}
	if q.OwnerID != u.ID {
		resp[ErrorField] = ErrorAuth
		return
	}
	// Sanitize the incoming quiz.  ALL FIELDS - including deep into questions
	// BE CAREFUL HERE.  Likely place to introduce xss vuln.
	q.Title = html.EscapeString(nq.Title)
	for i, qu := range nq.Questions {
		nq.Questions[i].Text = html.EscapeString(qu.Text)
		if !valid_atype[qu.AnswerType] {
			resp[ErrorField] = ErrorFormat
			c.Infof("Invalid answer type: ", qu.AnswerType)
		} else {
			nq.Questions[i].AnswerType = qu.AnswerType
		}
	}
	qm, _ := json.Marshal(nq.Questions)
	q.QuestionsM = string(qm)

	if _, err := datastore.Put(c, k, &q); err != nil {
		resp[ErrorField] = ErrorDatastore
		c.Infof("Could not put new quiz: %v", err)
	}
}

func quizCreateHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, u *user.User, resp map[string]interface{}) {
	qname := r.FormValue("qname")
	qname = html.EscapeString(qname)
	// validate
	q := &Quiz{qname, genQuizID(), time.Now(), u.ID, "", []Question{}}
	k := datastore.NewKey(c, "Quiz", q.ID, 0, nil)
	if _, err := datastore.Put(c, k, q); err != nil {
		resp[ErrorField] = ErrorDatastore
	}
}

func queryHandler(w http.ResponseWriter, r *http.Request) {
	resp := make(map[string]string)
	resp["val"] = r.FormValue("q")
	w.Header().Set("Content-Type", "text/javascript")
	if b, err := json.Marshal(resp); err == nil {
		w.Write(b)
	}
}

func quizGetHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, u *user.User, resp map[string]interface{}) {
	var q Quiz
	quizID := r.FormValue("q")
	k := datastore.NewKey(c, "Quiz", quizID, 0, nil)
	// sanity check quizID, please
	if err := datastore.Get(c, k, &q); err != nil {
		resp[ErrorField] = ErrorDatastore
		return
	}
	if q.OwnerID != u.ID {
		resp[ErrorField] = ErrorAuth
		return
	}
	qe := &Quiz{q.Title, q.ID, q.Created, "", "", []Question{}}
	json.Unmarshal([]byte(q.QuestionsM), &qe.Questions)
	resp["quiz"] = qe
}

func quizListHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, u *user.User, resp map[string]interface{}) {
	var qlist []Quiz = make([]Quiz, 0)
	q := datastore.NewQuery("Quiz").Filter("OwnerID =", u.ID).Order("Created")
	for t := q.Run(c); ; {
		var quiz Quiz
		_, err := t.Next(&quiz)
		if err == datastore.Done {
			break
		}
		if err != nil {
			c.Infof("datastore query failed: %v", err)
			resp[ErrorField] = ErrorDatastore
			break
		}
		qlist = append(qlist, quiz)
	}

	resp["quizlist"] = qlist
}

func adminHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	var p *Page
	if u := user.Current(c); u == nil {
		url, _ := user.LoginURL(c, "/admin")
		p = &Page{Title: "Authentication required",
			Content: fmt.Sprintf(`<a href="%s">Sign in or register</a>`, url)}
	} else {
		logouturl, _ := user.LogoutURL(c, "/admin")
		p = &Page{Title: "Admin",
			Content: fmt.Sprintf("Hi, %s, this is your admin page. <a href='%s'>logout</a> or <a href='/'>back to main</a>", u, logouturl)}
	}
	adminTemplate.Execute(w, p)
}
