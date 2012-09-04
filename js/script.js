/* Author: David G. Andersen
 *
 * requires jquery and jquery-ui to be loaded already
 */
"use strict";

/* Namespace for this app */
var sysquiz = {};

/* Status messages - @const */
sysquiz.FETCH_QUIZLIST = "Fetching quiz list";
sysquiz.CREATE_QUIZ = "Creating quiz";
sysquiz.FETCH_QUIZ = "Fetching quiz data";
sysquiz.SAVE_QUIZ = "Saving quiz";
sysquiz.DELETE_QUIZ = "Deleting quiz";

$(document).ready(function() {
    sysquiz.fetchQuizList();
    $("#createBtn").click(sysquiz.createQuiz);
    $("#saveBtn").click(sysquiz.saveQuiz);
    $("#newItem").click(sysquiz.newItem);
    $("#questionList").sortable();
});

sysquiz.newItem = function(event) {
    event.preventDefault();
    sysquiz.appendNewQuestion($('#questionList'));
}

sysquiz.parseQuestion = function(el) {
    var q = { Text: el.find('[name="questionText"]').val(),
	      AnswerType: el.find('[name="answerType"]').val(),
	      Work: el.find('[name="showWorkInput"]').val(),
	      ShowWork: el.find('[name="showWork"]').is(':checked'),
	      IsStop: el.find('[name="isStop"]').is(':checked')
	    };
    return q;
}

sysquiz.saveQuiz = function(event) {
    event.preventDefault();
    var q = { ID: $('#editID').val(),
	      Version: +($('#editVersion').val()), // + to convert to Int. sigh.
	      Title: $('#editName').val(),
	      Questions: new Array() };

    $('#questionList li').each(function(i, el) {
	q.Questions[i] = sysquiz.parseQuestion($(this));
    });
    var as_string = JSON.stringify(q);
    sysquiz.status(sysquiz.SAVE_QUIZ);
    $.post("/qu", {q: as_string}, sysquiz.saveQuizDone, "json");
}

sysquiz.saveQuizDone = function(r) {
    if (r["error"]) {
	alert("saving changes failed: " + r["error"].message);
    }
    sysquiz.removeStatus(sysquiz.SAVE_QUIZ);
    sysquiz.fetchQuizList();
    var ei = $("#editID").val();
    if (ei) {
	// design silliness:  we actually have all of this information
	// from fetchQuizList, we're just pretending it's different.
	// hrm.
	$.post("/qget", {q: ei}, sysquiz.editQuizGotQuizInfo, "json");
	// shouldn't this be a function call?
    }
}

sysquiz.createQuiz = function(event) {
    event.preventDefault();
    var neLabel = $("#createQuiz label#nameError");
    var nameInput = $('input#name');
    neLabel.hide();
    
    var qname = nameInput.val();
    var labelError = ""
    if (qname === "") {
	labelError = "Name can't be blank";
    }
    if (labelError !== "") {
	neLabel.text(labelError).show();
	nameInput.focus();
	return;
    }
    sysquiz.status(sysquiz.CREATE_QUIZ);
    $.post("/qc", {qname: qname }, sysquiz.createQuizDone, "json");
}

sysquiz.createQuizDone = function(r) {
    $("input#name").val('')
    sysquiz.removeStatus(sysquiz.CREATE_QUIZ)
    sysquiz.fetchQuizList();
}

sysquiz.fetchQuizList = function() {
    sysquiz.status(sysquiz.FETCH_QUIZLIST)
    $.post("/ql", {}, sysquiz.fetchQuizListDone, "json");
}

sysquiz.fetchQuizListDone = function(r) {
    sysquiz.removeStatus(sysquiz.FETCH_QUIZLIST)
    if (r['error']) {
	var e = r['error'];
	if (e.code === 401) {
	    $('#mainstatus').html("Need to login!  Go to <a href='/admin'>/admin</a> for now");
	} else {
	    $('#mainstatus').html("Some other error happened: " +e.Message);
	}
	return false
    }
    var ql = r['quizlist']
    var pageQl = $('#quizList');
    pageQl.empty();
    var qlEntry = $('#widgets .quizListEntry');
    for (var i = 0; i < ql.length; i++) {
	var el = qlEntry.clone(false).appendTo(pageQl);
	// THESE MUST HAPPEN AS .text(), otherwise we introduce XSS
	el.find(".quizName").text(ql[i].Title).unbind('click').click({id: ql[i].ID}, sysquiz.editQuiz);
	el.find(".quizCreated").text(ql[i].Created);
	el.find(".quizID").text(ql[i].ID);
	el.find(".quizAdmin").unbind('click').click({id: ql[i].ID}, sysquiz.adminQuiz);
    }
}

sysquiz.adminQuiz = function(event) {
    event.preventDefault();
    var quizID = event.data.id;
    $("div#edit").hide();
    $.post("/qget", {q: quizID}, sysquiz.adminQuizGotQuizInfo, "json");
}

sysquiz.adminQuizGotQuizInfo = function(r) {
    var quiz = r['quiz'];
    sysquiz.updatePage(quiz);
    $("div#admin").show();
    // Remove handler
    $("#toggleEnabled").unbind('click').click({q:quiz}, sysquiz.toggleEnabled);
}

// XXX:  This is lazy;  two AJAX instead of one.  Fix later.
sysquiz.toggleDone = function(r, quizID) {
    $.post("/qget", {q: quizID}, sysquiz.adminQuizGotQuizInfo, "json");
}

sysquiz.toggleEnabled = function(event) {
    var quiz = event.data.q;
    if (quiz.Enabled) {
	$.post("/qdisable", {q: quiz.ID}, 
	       function(data) { sysquiz.toggleDone(data, quiz.ID) }, "json")
    } else {
	$.post("/qenable", {q: quiz.ID}, function(data) { sysquiz.toggleDone(data, quiz.ID) }, "json")
    }
}

sysquiz.deleteQuiz = function(event) {
    event.preventDefault();
    var quiz = event.data.q;
    var really = confirm("Really delete quiz "+quiz.Title+" and all of its associated data and responses?");
    if (really) {
	sysquiz.status(sysquiz.DELETE_QUIZ);
	$("div#edit").hide()
	var as_string = JSON.stringify(quiz);
	// silly, but resending the full quiz lets us get the version easily.
	$.post("/qdel", {q: as_string}, sysquiz.deleteQuizDone, "json");
    }
}

sysquiz.deleteQuizDone = function(r) {
    sysquiz.removeStatus(sysquiz.REMOVE_QUIZ);
    if (r["error"]) {
	alert("Could not delete quiz: " + r["error"].message);
    }
    sysquiz.fetchQuizList();
}

sysquiz.editQuiz = function(event) {
    event.preventDefault();
    var quizID = event.data.id;
    sysquiz.status(sysquiz.FETCH_QUIZ);
    $.post("/qget", {q: quizID}, sysquiz.editQuizGotQuizInfo, "json");
}

sysquiz.appendNewQuestion = function(ql) {
    var qEntry = $('#widgets .questionInput');
    var el = qEntry.clone(false).appendTo(ql);
    el.find(".removeQBtn").unbind('click').click({e: el}, function(event) {
	event.data.e.remove();
	return false;
    });
    el.find('[name="answerType"]').change({e: el}, function(event) {
	sysquiz.changeAnswerType(event.data.e);
    });
    el.find('[name="showWork"]').change({e: el}, function(event) {
	sysquiz.updateAnswerDisplay(event.data.e);
    });
    el.find('[name="isStop"]').change({e: el}, function(event) {
	sysquiz.updateAnswerDisplay(event.data.e);
    });
    
    sysquiz.changeAnswerType(el);
    sysquiz.updateAnswerDisplay(el);

    //el.find("#textAnswer").resizable({ handles: "se", disabled: false });
    return el;
}

sysquiz.changeAnswerType = function(el) {
    //el.find('.answerbox').hide();
    var atype = el.find('[name="answerType"]').val();
    // multiple choice isn't handled yet.
    // I want them to be able to resize the text input box for answer.
    // and save it.
}

sysquiz.updatePage = function(quiz) {
    $('#enabledStatus').text(quiz.Enabled ? "Enabled" : "Disabled");
    $('#toggleEnabled').text("Click to " + (quiz.Enabled ? "Disable" : "Enable"));
}

sysquiz.editQuizGotQuizInfo = function(r) {
    var quiz = r['quiz'];
    sysquiz.updatePage(quiz);
    $("#revertBtn").unbind('click').click({id: quiz.ID}, sysquiz.editQuiz);
    $('#editName').val(quiz.Title);
    $('#editID').val(quiz.ID);
    $('#editVersion').val(quiz.Version);
    $('#edit').show();
    var ql = $('#questionList');
    $("#quizDelete").unbind('click').click({q: quiz}, sysquiz.deleteQuiz);
    ql.empty();
    if (quiz.Questions) {
	for (var i = 0; i < quiz.Questions.length; i++) {
	    var q = quiz.Questions[i];
	    var el = sysquiz.appendNewQuestion(ql);
	    el.find('[name="questionText"]').val(q.Text);
	    el.find('[name="answerType"]').val(q.AnswerType);
	    el.find('[name="isStop"]').prop("checked", q.IsStop);
	    el.find('[name="showWork"]').prop("checked", q.ShowWork);
	    el.find('[name="showWorkInput"]').val(q.Work);
	    sysquiz.updateAnswerDisplay(el)
	    // throw in an answer div so we can delete the whole thing
	    // if they change the type...
	    // and then appendAnswerDuration(el)
	}
    }
}

sysquiz.updateAnswerDisplay = function(el) {
    el.find('[name="showWorkDiv"]').toggle(el.find('[name="showWork"]').is(':checked'));

    // xxx - don't like that this knows so much about the CSS.
    if (el.find('[name="isStop"]').is(':checked')) {
	el.css("border-bottom", "8px solid grey");
    } else {
	el.css("border-bottom", "2px dashed grey");
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
    el.append($('#widgets #durationUnits'));
}