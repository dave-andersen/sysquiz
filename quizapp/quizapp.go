package myapp

import (
	"appengine"
	"appengine/datastore"
	"appengine/user"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"net/http"
	"text/template"
	"time"
	"errors"
)

type Page struct {
	Title   string
	Content string
	Logout  string
}

type Question struct {
	Text          string
	AnswerOptions []string // for multiple choice type questions
	Answer        string
	AnswerType    string // "text", "int", "float", ...
	IsStop        bool   // Must they answer this question before going on?
	ShowWork      bool   // Give them a 'show your work' dialog?
	Work          string

}

type Quiz struct {
	Title       string
	ID          string
	Key         string `json:"-"` // Don't disclose this.
	Created     time.Time
	OwnerID     string `json:"-"` // ownerID not sent via JSON
	QuestionsM  string `json:"-"`// marshaled into JSON for storage in the datastore
	Questions   []Question `datastore:"-"` // sent via the network
	Version     int // for multiple writer consistency
	Enabled     bool
}

type Answer struct {
	Response string
	Submitted bool // Have they "submitted" this answer and can no longer modify?
}

type QuizRecord struct {
	ID       string
	Name     string
	QuizID   string
	AnswersM string `json:"-"` // marshaled into JSON for storage in the datastore
	Answers []Answer `datastore:"-"` // sent via the network
	Version  int // Multiple-writer consistency, as in the quiz
}

// Pared down verisons that do not export data we want hidden.
type StudentQuiz struct {
	Title string
	Questions []Question
	NumQuestions int // The questions can be partial
}

type StudentQuizRecord struct {
	ID string
	Name string
	Answers []Answer
	Version int
}


type JSONError struct {
	Name string `json:"name"`
	Code int `json:"code"`
	Message string `json:"message"`
	Error interface{} `json:"error"`
}

var (
	indexTemplate, createTemplate, pageTemplate, adminTemplate, takeTemplate *template.Template

	// Error codes to use in the JSON response
	ErrorAuth = JSONError{"AUTH", 401, "Authentication required", nil}
	ErrorDatastore = JSONError{"DATA", 510, "Internal Datastore Error", nil}
	ErrorFormat = JSONError{"FORMAT", 400, "Bad request", nil}
	ErrorVersion = JSONError{"VERSION", 409, "Conflicting update - someone else updated this since you loaded it.", nil}
	ErrorOther = JSONError{"OTHER", 500, "Other error", nil}
	ErrorQuizDisabled = JSONError{"DISABLED", 402, "Quiz is disabled", nil}

	valid_atype = map[string]bool { "text" : true, "mc": true, "int": true, "float": true, "duration": true }
)

const (
	ErrorField = "error"
)

func genQuizID() string {
	b := make([]byte, 16)
	if n, err := rand.Read(b); n != 16 {
		panic(fmt.Sprintf("could not read 16 random bytes.  I'm very unhappy: %s", err))
	}
	return base32.StdEncoding.EncodeToString(b)[0:24]
}

func init() {
	pageTemplate = template.Must(template.ParseFiles("html/page.html", "html/boring.html"))
	adminTemplate = template.Must(template.ParseFiles("html/page.html", "html/admin.html"))
	createTemplate = template.Must(template.ParseFiles("html/page.html", "html/create_inner.html"))
	indexTemplate = template.Must(template.ParseFiles("html/page.html", "html/index_inner.html"))
	takeTemplate = template.Must(template.ParseFiles("html/takepage.html", "html/take.html"))
	http.HandleFunc("/", indexHandler);
	// Functions for Instructors
	http.HandleFunc("/create", createHandler);
	http.Handle("/ql", AuthHandlerFunc(quizListHandler))
	http.Handle("/qc", AuthHandlerFunc(quizCreateHandler))
	http.Handle("/qget", AuthHandlerFunc(quizGetHandler))
	http.Handle("/qu", AuthHandlerFunc(quizUpdateHandler))
	http.Handle("/qdel", AuthHandlerFunc(quizDeleteHandler))
	http.Handle("/qenable", AuthHandlerFunc(quizEnableHandler))
	http.Handle("/qdisable", AuthHandlerFunc(quizDisableHandler))
	http.Handle("/qrget", AuthHandlerFunc(quizGetQuizRecords))
	http.Handle("/qrcreate", AuthHandlerFunc(quizCreateQuizRecords))
	// Functions for Students
	http.HandleFunc("/take", takeHandler);
	http.Handle("/take/qget", NoAuthHandlerFunc(quizStudentGetHandler))
	http.Handle("/take/save", NoAuthHandlerFunc(quizStudentSaveHandler))
}

type NoAuthHandlerFunc func(http.ResponseWriter, *http.Request, appengine.Context, map[string]interface{})

func (f NoAuthHandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	jsonHandler(w, r, f)
}

func jsonHandler(w http.ResponseWriter, r *http.Request, handler func(http.ResponseWriter, *http.Request, appengine.Context, map[string]interface{})) {
	resp := make(map[string]interface{})
	c := appengine.NewContext(r)
	handler(w, r, c, resp)
	w.Header().Set("Content-Type", "text/javascript")
	b, _ := json.Marshal(resp)
	w.Write(b)
	return
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

func quizDeleteHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, u *user.User, resp map[string]interface{}) {
	var nq Quiz
	if err := json.Unmarshal([]byte(r.FormValue("q")), &nq); err != nil {
		c.Infof("Unmarshal json failed on %v", r.FormValue("q"))
		resp[ErrorField] = ErrorOther
		return
	}
	quizID := nq.ID
	k := datastore.NewKey(c, "Quiz", quizID, 0, nil)
	err := datastore.RunInTransaction(c, func(c appengine.Context) error {
		var q Quiz
		if err1 := datastore.Get(c, k, &q); err1 != nil {
			resp[ErrorField] = ErrorDatastore
			return err1
		}
		if q.OwnerID != u.ID {
			resp[ErrorField] = ErrorAuth
			return errors.New("Owner mismatch")
		}
		if (q.Version != nq.Version) {
			resp[ErrorField] = ErrorVersion
			return errors.New("Version mismatch")
		}
		if err1 := datastore.Delete(c, k); err1 != nil {
			resp[ErrorField] = ErrorDatastore
			return err1
		}
		return nil
	}, nil) // xxx - this function is kinda long for an inline one...
	if err != nil {
		c.Infof("Transactional update failed: ", err)
	}
}

func quizEnableHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, u *user.User, resp map[string]interface{}) {
	enableInternal(w, r, c, u, resp, true)
}
func quizDisableHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, u *user.User, resp map[string]interface{}) {
	enableInternal(w, r, c, u, resp, false)
}

func enableInternal(w http.ResponseWriter, r *http.Request, c appengine.Context, u *user.User, resp map[string]interface{}, enabled bool) {
	quizID := r.FormValue("q")
	k := datastore.NewKey(c, "Quiz", quizID, 0, nil)
	// sanity check quizID, please
	err := datastore.RunInTransaction(c, func(c appengine.Context) error {
		var q Quiz
		if err1 := datastore.Get(c, k, &q); err1 != nil {
			resp[ErrorField] = ErrorDatastore
			return err1
		}
		if q.OwnerID != u.ID {
			resp[ErrorField] = ErrorAuth
			return errors.New("Owner mismatch")
		}
		
		q.Enabled = enabled
		// We store unescaped data.  Javascript _must_ put into the .text()
		// or .val() of a field, not the .html().

		if _, err1 := datastore.Put(c, k, &q); err1 != nil {
			resp[ErrorField] = ErrorDatastore
			c.Infof("Could not put new quiz: %v", err1)
			return errors.New("put failed")
		}
		return nil
	}, nil)
	if err != nil {
		c.Infof("Transactional update failed: ", err)
	}
}


func quizUpdateHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, u *user.User, resp map[string]interface{}) {
	var nq Quiz
	if err := json.Unmarshal([]byte(r.FormValue("q")), &nq); err != nil {
		c.Infof("Unmarshal json failed on %v", r.FormValue("q"))
		resp[ErrorField] = ErrorOther
		return
	}
	quizID := nq.ID
	k := datastore.NewKey(c, "Quiz", quizID, 0, nil)
	// sanity check quizID, please
	err := datastore.RunInTransaction(c, func(c appengine.Context) error {
		// Rule:  'q' is the quiz from the datastore.  'nq' is the quiz
		// from the app.  Fields to be updated must be pulled explicitly
		// from nq into q.  'q' is put back into the datastore.
		var q Quiz
		if err1 := datastore.Get(c, k, &q); err1 != nil {
			resp[ErrorField] = ErrorDatastore
			return err1
		}
		if q.OwnerID != u.ID {
			resp[ErrorField] = ErrorAuth
			return errors.New("Owner mismatch")
		}
		if (q.Version != nq.Version) {
			resp[ErrorField] = ErrorVersion
			return errors.New("Version mismatch")
		}
		q.Version++
		
		// We store unescaped data.  Javascript _must_ put into the .text()
		// or .val() of a field, not the .html().
		q.Title = nq.Title
		for i, qu := range nq.Questions {
			if !valid_atype[qu.AnswerType] {
				resp[ErrorField] = ErrorFormat
				c.Infof("Invalid answer type: ", qu.AnswerType)
				return errors.New("bad format")
			} else {
				nq.Questions[i].AnswerType = qu.AnswerType
			}
		}
		qm, _ := json.Marshal(nq.Questions)
		q.QuestionsM = string(qm)

		if _, err1 := datastore.Put(c, k, &q); err1 != nil {
			resp[ErrorField] = ErrorDatastore
			c.Infof("Could not put new quiz: %v", err1)
			return errors.New("put failed")
		}
		return nil
	}, nil) // xxx - this function is kinda long for an inline one...
	if err != nil {
		c.Infof("Transactional update failed: ", err)
	}
}

func quizCreateHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, u *user.User, resp map[string]interface{}) {
	qname := r.FormValue("qname")
	// validate
	q := &Quiz{qname, genQuizID(), genQuizID(), time.Now(), u.ID, "", []Question{}, 0, false}
	k := datastore.NewKey(c, "Quiz", q.ID, 0, nil)
	if _, err := datastore.Put(c, k, q); err != nil {
		resp[ErrorField] = ErrorDatastore
	}
}

func quizGetHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, u *user.User, resp map[string]interface{}) {
	quizID := r.FormValue("q")
	q, err := getQuizIfOwnerMatch(quizID, c, u)
	if err != nil {
		resp[ErrorField] = err
		return
	}
	json.Unmarshal([]byte(q.QuestionsM), &q.Questions)
	resp["quiz"] = q
}

func quizListHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, u *user.User, resp map[string]interface{}) {
	qlist := make([]Quiz, 0)
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

func takeHandler(w http.ResponseWriter, r *http.Request) {
	p := &Page{Title: "Take Quiz"}
	takeTemplate.Execute(w, p)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	p := &Page{Title: "Quizzer"}
	indexTemplate.Execute(w, p)
}

func createHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	var p *Page
	if u := user.Current(c); u == nil {
		url, _ := user.LoginURL(c, "/create")
		p = &Page{Title: "Authentication required",
			Content: fmt.Sprintf(`<a href="%s">Sign in or register</a>`, url)}
		adminTemplate.Execute(w, p)
	} else {
		logouturl, _ := user.LogoutURL(c, "/")
		p = &Page{Title: "Quizzer 0.00000gamma",
		Logout: fmt.Sprintf("%s (<a href='%s'>logout</a>)", u, logouturl)}
		createTemplate.Execute(w, p)
	}
}

func getQuizIfOwnerMatch(quizID string, c appengine.Context, u *user.User) (q Quiz, err *JSONError) {
	k := datastore.NewKey(c, "Quiz", quizID, 0, nil)
	if err := datastore.Get(c, k, &q); err != nil {
		return q, &ErrorDatastore
	}
	if q.OwnerID != u.ID {
		return q, &ErrorAuth
	}
	
	return q, nil
}

// Administering quiz functions
func quizGetQuizRecords(w http.ResponseWriter, r *http.Request, c appengine.Context, u *user.User, resp map[string]interface{}) {
	quizID := r.FormValue("q")
	q, err := getQuizIfOwnerMatch(quizID, c, u)
	if err != nil {
		resp[ErrorField] = err
		return
	}

	qidlist := make([]QuizRecord, 0)
	qu := datastore.NewQuery("QuizRecord").Filter("QuizID =", q.ID).Order("Name")
	for t := qu.Run(c); ; {
		var qr QuizRecord
		_, err := t.Next(&qr)
		if err == datastore.Done {
			break
		}
		if err != nil {
			c.Infof("datastore query failed: %v", err)
			resp[ErrorField] = ErrorDatastore
			break
		}
		qidlist = append(qidlist, qr)
	}

	resp["quizRecordList"] = qidlist
}

func quizCreateQuizRecords(w http.ResponseWriter, r *http.Request, c appengine.Context, u *user.User, resp map[string]interface{}) {
	quizID := r.FormValue("q")
	var qrecs []string
	if err := json.Unmarshal([]byte(r.FormValue("recs")), &qrecs); err != nil {
		c.Infof("Unmarshal json failed on %v", r.FormValue("recs"))
		resp[ErrorField] = ErrorOther
		return
	}

	q, err := getQuizIfOwnerMatch(quizID, c, u)
	if err != nil {
		resp[ErrorField] = err
		return
	}

	for _, rec := range qrecs {
		qr := &QuizRecord{genQuizID(), rec, q.ID, "", nil, 0}
		k := datastore.NewKey(c, "QuizRecord", qr.ID, 0, nil)
		if _, err := datastore.Put(c, k, qr); err != nil {
			resp[ErrorField] = ErrorDatastore
			return
		}
	}
}

// Student test-taking functions

func getQuizAndRecord(c appengine.Context, quizRecordID string) (q Quiz, qr QuizRecord, errorReturn *JSONError) {
	errorReturn = nil
	k := datastore.NewKey(c, "QuizRecord", quizRecordID, 0, nil)
	if err := datastore.Get(c, k, &qr); err != nil {
		errorReturn = &ErrorDatastore
		return
	}

	quizID := qr.QuizID
	quizKey := datastore.NewKey(c, "Quiz", quizID, 0, nil)
	if err := datastore.Get(c, quizKey, &q); err != nil {
		errorReturn = &ErrorDatastore
		return
	}

	if (!q.Enabled) {
		errorReturn = &ErrorQuizDisabled
		return
	}
	
	json.Unmarshal([]byte(qr.AnswersM), &qr.Answers)
	json.Unmarshal([]byte(q.QuestionsM), &q.Questions)
	return
}

func quizStudentGetHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, resp map[string]interface{}) {
	q, qr, err := getQuizAndRecord(c, r.FormValue("qr"))
	if err != nil {
		resp[ErrorField] = *err
		return
	}

	// Exported version
	sqr := &StudentQuizRecord{qr.ID, qr.Name, qr.Answers, qr.Version}

	// Some questions may be hidden until the answers have been submitted.
	qlist := make([]Question, 0)
	numAnswers := len(qr.Answers)
	isSubmitted := true
	for i := 0; i < len(q.Questions); i++ {
		if (i >= numAnswers || !qr.Answers[i].Submitted) {
			isSubmitted = false
		}
		qlist = append(qlist, q.Questions[i])
		if q.Questions[i].IsStop && !isSubmitted {
			break
		}
	}

	sq := &StudentQuiz{q.Title, qlist, len(q.Questions)}
	resp["quizRecord"] = sqr
	resp["quiz"] = sq
}

func quizStudentSaveHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, resp map[string]interface{}) {
	var qr, nqr QuizRecord
	if err := json.Unmarshal([]byte(r.FormValue("qr")), &nqr); err != nil {
		c.Infof("Unmarshal json failed on %v", r.FormValue("qr"))
		resp[ErrorField] = ErrorOther
		return
	}

	// We're just calling this to check if the quiz is enabled and valid.
	_, _, quizErr := getQuizAndRecord(c, r.FormValue("qr"))
	if quizErr != nil {
		resp[ErrorField] = *quizErr
		return
	}

	k := datastore.NewKey(c, "QuizRecord", nqr.ID, 0, nil)

	err := datastore.RunInTransaction(c, func(c appengine.Context) error {
		if err := datastore.Get(c, k, &qr); err != nil {
			resp[ErrorField] = ErrorDatastore
			return errors.New("Datastore Error")
		}
		json.Unmarshal([]byte(qr.AnswersM), &qr.Answers)

		// Version check
		if (qr.Version != nqr.Version) {
			resp[ErrorField] = ErrorVersion
			return errors.New("Version mismatch");
		}
		// They're not allowed to overwrite previously-submitted answers
		numAnswers := len(qr.Answers)
		for i, an := range nqr.Answers {
			if (i >= numAnswers) {
				qr.Answers = append(qr.Answers, an)
			} else {
				if (qr.Answers[i].Submitted) {
					continue
				} else {
					qr.Answers[i].Response = an.Response
					qr.Answers[i].Submitted = an.Submitted
				}
			}
		}
		am, _ := json.Marshal(qr.Answers)
		qr.AnswersM = string(am)
		if _, err1 := datastore.Put(c, k, &qr); err1 != nil {
			resp[ErrorField] = ErrorDatastore
			c.Infof("Could not put quiz record: %v", err1)
			return errors.New("save failed")
		}
		
		return nil
	}, nil);
	if err != nil {
		c.Infof("Transactional record save failed: ", err)
	}		
}