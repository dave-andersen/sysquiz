/* Author: David G. Andersen
 *
 * requires jquery to be loaded already
 */

var FETCH_QUIZLIST = "Fetching quiz list";
var CREATE_QUIZ = "Creating quiz";
var FETCH_QUIZ = "Fetching quiz data";
var SAVE_QUIZ = "Saving quiz";
var userid = "userid";

/* Namespace for this app */
var sysquiz = {};

$(document).ready(function()
{
    sysquiz.fetchQuizList();
    $("#createBtn").click(sysquiz.createQuiz);
    $("#saveBtn").click(sysquiz.saveQuiz);
    $("#newitem").click(sysquiz.newItem);
    $("#questionlist").sortable();
});

sysquiz.newItem = function(event) {
    event.preventDefault();
    sysquiz.appendNewQuestion($('#questionlist'));
}

sysquiz.parseQuestion = function(el) {
    var q = { Text: el.find('#questionText').val(),
	      AnswerType: el.find('#answerType').val() };
    return q;
}

sysquiz.saveQuiz = function(event) {
    event.preventDefault();
    var q = { ID: $('#editId').val(),
	      Version: +($('#editVersion').val()), // + to convert to Int. sigh.
	      Title: $('#editName').val(),
	      Questions: new Array() };

    $('#questionlist li').each(function(i, el) {
	q.Questions[i] = sysquiz.parseQuestion($(this));
    });
    var as_string = JSON.stringify(q);
    sysquiz.status(SAVE_QUIZ);
    $.post("/qu", {q: as_string}, sysquiz.saveQuizDone, "json");
}

sysquiz.saveQuizDone = function(r) {
    if (r["error"]) {
	alert("saving changes failed: " + r["error"].message);
    }
    sysquiz.removeStatus(SAVE_QUIZ);
    sysquiz.fetchQuizList();
}

sysquiz.createQuiz = function(event) {
    event.preventDefault();
    var neLabel = $("#createquiz label#nameError");
    var nameInput = $('input#name');
    neLabel.hide();
    
    var qname = nameInput.val();
    var labelError = ""
    if (qname == "") {
	labelError = "Name can't be blank";
    }
    if (labelError != "") {
	neLabel.text(labelError).show();
	nameInput.focus();
	return;
    }
    sysquiz.status(CREATE_QUIZ);
    $.post("/qc", {qname: qname }, sysquiz.createQuizDone, "json");
}

sysquiz.createQuizDone = function(r) {
    $("input#name").val('')
    sysquiz.removeStatus(CREATE_QUIZ)
    sysquiz.fetchQuizList();
}

sysquiz.fetchQuizList = function() {
    sysquiz.status(FETCH_QUIZLIST)
    $.post("/ql", {u: userid }, sysquiz.fetchQuizListDone, "json");
}

sysquiz.fetchQuizListDone = function(r) {
    sysquiz.removeStatus(FETCH_QUIZLIST)
    if (r['error']) {
	var e = r['error'];
	if (e.code == 401) {
	    $('#mainstatus').html("Need to login!  Go to <a href='/admin'>/admin</a> for now");
	} else {
	    $('#mainstatus').html("Some other error happened: " +e.Message);
	}
	return false
    }
    var ql = r['quizlist']
    var pageQl = $('#quizlist');
    pageQl.empty();
    for (var i = 0; i < ql.length; i++) {
	var editlink = "editlink_"+ql[i].ID;
	pageQl.append('<li><a href="#edit" id="'+editlink+'">'+ql[i].Title+'</a> - '+ql[i].ID+' ('+ql[i].Created+')</li>');
	pageQl.find('#'+editlink).click({id: ql[i].ID}, sysquiz.editQuiz);
    }
}

sysquiz.editQuiz = function(event) {
    event.preventDefault();
    var quizID = event.data.id;
    sysquiz.status(FETCH_QUIZ);
    $.post("/qget", {q: quizID}, sysquiz.editQuizGotQuizInfo, "json");
}

sysquiz.appendNewQuestion = function(ql) {
    var qhtml = $('#questionInput').html()
    ql.append(qhtml);
    var el = ql.find('li').last();  // xxx - this is n^2. :(
    el.find("#removeQBtn").click({e: el}, function(event) {
	event.data.e.remove();
	return false;
    });
    el.find("#answerType").change({e: el}, function(event) {
	sysquiz.changeAnswerType(event.data.e);
    });
    sysquiz.changeAnswerType(el);
    //el.find("#textAnswer").resizable({ handles: "se", disabled: false });
    return el;
}

sysquiz.changeAnswerType = function(el) {
    //el.find('.answerbox').hide();
    var atype = el.find("#answerType").val();
    // multiple choice isn't handled yet.
    // I want them to be able to resize the text input box for answer.
    // and save it.
}

sysquiz.editQuizGotQuizInfo = function(r) {
    var quiz = r['quiz'];
    $("#revertBtn").unbind('click').click({id: quiz.ID}, sysquiz.editQuiz);
    $('#editName').val(quiz.Title);
    $('#editId').val(quiz.ID);
    $('#editVersion').val(quiz.Version);
    $('#edit').show();
    var ql = $('#questionlist');
    ql.empty();
    if (quiz.Questions) {
	for (var i = 0; i < quiz.Questions.length; i++) {
	    var q = quiz.Questions[i];
	    var el = sysquiz.appendNewQuestion(ql);
	    el.find("#questionText").val(q.Text)
	    // throw in an answer div so we can delete the whole thing
	    // if they change the type...
	    // and then appendAnswerDuration(el)
	}
    }
}

sysquiz.status = function(s) {
    $('#status').text(s)
}
sysquiz.removeStatus = function(s) {
    $('#status').text('')
}

// Create HTML snippets for different answer types and bind them
// to appropriate validators
sysquiz.appendAnswerDuration = function(el) {
    el.append($('#widgets #duration'));
    el.append($('#widgets #DurationUnits'));
}