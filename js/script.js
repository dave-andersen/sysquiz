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
    $("#create_btn").click(sysquiz.createQuiz);
    $("#save_btn").click(sysquiz.saveQuiz);
    $("#newitem").click(sysquiz.newItem);
    $("#questionlist").sortable();
});

sysquiz.newItem = function(event) {
    event.preventDefault();
    sysquiz.appendNewQuestion($('#questionlist'));
}

sysquiz.parseQuestion = function(el) {
    var q = { Text: el.find('#question_text').val(),
	      AnswerType: el.find('#AnswerType').val() };
    return q;
}

sysquiz.saveQuiz = function(event) {
    event.preventDefault();
    var q = { ID: $('#edit_id').val(),
	      Title: $('#edit_name').val(),
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
	alert("saving changes failed: " + r["error"].Message);
    }
    sysquiz.removeStatus(SAVE_QUIZ);
    sysquiz.fetchQuizList();
}

sysquiz.createQuiz = function(event) {
    event.preventDefault();
    var neLabel = $("#createquiz label#name_error");
    var nameInput = $('input#name');
    neLabel.hide();
    
    var qname = nameInput.val();
    var label_error = ""
    if (qname == "") {
	label_error = "Name can't be blank";
    }
    if (label_error != "") {
	neLabel.text(label_error).show();
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
	$('#'+editlink).click({id: ql[i].ID}, sysquiz.editQuiz);
    }
}

sysquiz.editQuiz = function(event) {
    event.preventDefault();
    var quizID = event.data.id;
    sysquiz.status(FETCH_QUIZ);
    $.post("/qget", {q: quizID}, sysquiz.editQuizGotQuizInfo, "json");
}

sysquiz.appendNewQuestion = function(ql) {
    var qhtml = $('#question_input').html()
    ql.append(qhtml);
    var el = ql.find('li').last();  // xxx - this is n^2. :(
    el.find("#remove_q_btn").click({e: el}, function(event) {
	event.data.e.remove();
	return false;
    });
    return el;
}

sysquiz.editQuizGotQuizInfo = function(r) {
    var quiz = r['quiz'];
    $("#revert_btn").unbind('click').click({id: quiz.ID}, sysquiz.editQuiz);
    $('#edit_name').val(quiz.Title);
    $('#edit_id').val(quiz.ID);
    $('#edit').show();
    var ql = $('#questionlist');
    ql.empty();
    if (quiz.Questions) {
	for (var i = 0; i < quiz.Questions.length; i++) {
	    var q = quiz.Questions[i];
	    var el = sysquiz.appendNewQuestion(ql);
	    el.find("#question_text").val(q.Text)
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