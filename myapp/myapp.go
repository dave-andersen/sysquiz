package myapp

import (
	"net/http"
	"text/template"
	"encoding/json"
	"crypto/rand"
	"encoding/base32"
	"html"
	"fmt"
	"appengine"
	"appengine/datastore"
	"appengine/user"
	"time"
)

type Page struct {
     Title string
     Content string
}

type Question struct {
	Text string
	Answer_Options []string // for multiple choice type questions
	Answer string
	Answer_Type string // "text", "int", "float", ...
	Is_Stop bool // Must they answer this question before going on?
}

type Quiz struct {
	Title string
	ID string
	Created time.Time
	OwnerID string  `json:"-"`  // ownerID not sent via JSON
	Questions_m string // marshaled into JSON for storage in the datastore
}

type Quize struct { // ugly.  just a Quiz, but with Questions in correct format.
	Title string
	ID string
	Created time.Time
	OwnerID string  `json:"-"`  // ownerID not sent via JSON
	Questions []Question
}

var (
	pageTemplate = template.Must(template.ParseFiles("page.html", "boring.html"))
	adminTemplate = template.Must(template.ParseFiles("page.html", "admin.html"))
	instanceHitCount = 0
)


func genQuizID() string {
	b := make([]byte, 16)
	n, err := rand.Read(b)
	if n != 16 {
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
	w.Header().Set("Content-Type", "text/javascript")
	c := appengine.NewContext(r)
	u := user.Current(c)
	resp := make(map[string]interface{})
	if u == nil {
		c.Infof("jsonAuthHandler punting:  no valid user for %v", r.URL)
		resp["status"] = "AUTH"
	} else {
		handler(w, r, c, u, resp)
	}
	b, _ := json.Marshal(resp)
	w.Write(b)
	return
}

func quizUpdateHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, u *user.User, resp map[string]interface{}) {
	var q Quiz;
	var nq Quize;
	if err := json.Unmarshal([]byte(r.FormValue("q")), &nq); err != nil {
		c.Infof("Unmarshal json failed on %v", r.FormValue("q"))
		resp["status"] = "json failed";
		return;
	}
	quizID := nq.ID;
	k := datastore.NewKey(c, "Quiz", quizID, 0, nil)
	// sanity check quizID, please
	if err := datastore.Get(c, k, &q); err != nil {
		resp["status"] = "Failed";
		return
	}
	if q.OwnerID != u.ID {
		resp["status"] = "AUTH";
		return
	}
	// Sanitize the incoming quiz.  ALL FIELDS - including deep into questions
	// BE CAREFUL HERE.  Likely place to introduce xss vuln.
	q.Title = html.EscapeString(nq.Title);
	for i, qu := range nq.Questions {
		nq.Questions[i].Text = html.EscapeString(qu.Text)
	}
	qm, _ := json.Marshal(nq.Questions)
	q.Questions_m = string(qm)

	if _, err := datastore.Put(c, k, &q); err == nil {
		resp["status"] = "ok"
	} else {
		c.Infof("Could not put new quiz: %v", err)
		resp["status"] = "failed"
	}
}

func quizCreateHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, u *user.User, resp map[string]interface{}) {
	qname := r.FormValue("qname")
	qname = html.EscapeString(qname)
	// validate
	q := &Quiz{qname, genQuizID(), time.Now(), u.ID, ""}
	k := datastore.NewKey(c, "Quiz", q.ID, 0, nil)
	if _, err := datastore.Put(c, k, q); err == nil {
		resp["status"] = "ok"
	} else {
		resp["status"] = "failed"
	}
}

func queryHandler(w http.ResponseWriter, r *http.Request) {
	resp := make(map[string]string)
	resp["val"] = r.FormValue("q")
	w.Header().Set("Content-Type", "text/javascript")
	b, err := json.Marshal(resp)
	if err == nil {
		w.Write(b)
	}
}

func quizGetHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, u *user.User, resp map[string]interface{}) {
	var q Quiz
	quizID := r.FormValue("q")
	k := datastore.NewKey(c, "Quiz", quizID, 0, nil)
	// sanity check quizID, please
	if err := datastore.Get(c, k, &q); err != nil {
		resp["status"] = "Failed";
		return
	}
	if q.OwnerID != u.ID {
		resp["status"] = "AUTH";
		return
	}
	qe := &Quize{q.Title, q.ID, q.Created, "", []Question{}}
	json.Unmarshal([]byte(q.Questions_m), &qe.Questions)
	resp["status"] = "ok"
	resp["quiz"] = qe
}

func quizListHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, u *user.User, resp map[string]interface{}) {
	var qlist []Quiz = make([]Quiz, 0)
	var status = "ok"
	q := datastore.NewQuery("Quiz").Filter("OwnerID =", u.ID).Order("Created")
	for t := q.Run(c); ; {
		var quiz Quiz
		_, err := t.Next(&quiz)
		if err == datastore.Done {
			break
		}
		if err != nil {
			c.Infof("datastore query failed: %v", err)
			status = "failed"
			break
		}
		qlist = append(qlist, quiz)
	}

	resp["quizlist"] = qlist
	resp["status"] = status
}

func adminHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	u := user.Current(c)
	var p *Page
	if u == nil {
		url, _ := user.LoginURL(c, "/admin")
		p = &Page{Title: "Authentication required",
		Content: fmt.Sprintf(`<a href="%s">Sign in or register</a>`, url)}
	} else {
		logouturl, _ := user.LogoutURL(c, "/admin")
		p = &Page{Title: "Admin",
		Content: fmt.Sprintf("Hi, %s, this is your admin page. <a href='%s'>logout</a> or <a href='/'>back to main</a>",u, logouturl) }
	}
	adminTemplate.Execute(w, p)
}
