/* Author:  David G. Andersen
* 
* requires jquery and jquery-ui to be loaded already
*/

"use strict";

/* namespace for this app */
var sysquiz = {};

$(document).ready(function() {
    sysquiz.takeQuiz();
    $('#saveBtn').click(sysquiz.takeSaveWork);
    $('#revertBtn').click(sysquiz.takeRevert);
});

sysquiz.takeSaveWork = function(event) {
    event.preventDefault();
    var qr = { ID: myQR.ID,
	       Version: myQR.Version,
	       Answers: new Array() };
    $('#takeQuestionList li').each(function(i, el) {
	var ans = $(this).find('[name="answerText"]').val();
	// Get the submitted status...
	var submitted = $(this).find('[name="finalizeCheckbox"]').is(':checked');
	qr.Answers[i] = { Response: ans, Submitted: submitted };
    });
    var as_string = JSON.stringify(qr);
    $.post("/take/save", {qr: as_string}, sysquiz.takeSaveDone, "json");
}

sysquiz.takeSaveDone = function(r) {
    if (r["error"]) {
	alert("Saving changes failed: " + r["error"].message + ".  Suggest copy/pasting your work somewhere.");
    } 
    sysquiz.takeQuiz();
}

sysquiz.takeRevert = function(event) {
    event.preventDefault();
}

var myQR = {};

sysquiz.takeQuiz = function() {
    myQR.ID = getURLParameter("q");
    $.post("/take/qget", {qr: myQR.ID}, sysquiz.takeGotQuizData, "json");
}

sysquiz.takeGotQuizData = function(r) {
    if (r['error']) {
	var e = r['error'];
	alert("Could not load quiz: " +e);
	return;
    }

    var quiz = r['quiz'];
    var quizRecord = r['quizRecord'];

    $('#quizName').text(quiz.Title + " (" + quiz.NumQuestions + " questions)");
    if (quiz.NumQuestions > quiz.Questions.length) {
	$('#moreQuestions').show();
    } else {
	$('#moreQuestions').hide();
    }
    
    var ql = $('#takeQuestionList');
    var qEntry = $('#widgets .questionInput');
    var qStaticEntry = $('#widgets .questionDisplay');
    ql.empty();
    for (var i = 0; i < quiz.Questions.length; i++) {
	var response = "";
	var submitted = false;
	if (quizRecord.Answers && i < quizRecord.Answers.length) {
	    response = quizRecord.Answers[i].Response;
	    submitted = quizRecord.Answers[i].Submitted;
	}
	var q = quiz.Questions[i];
	var el;
	if (submitted) {
	    el = qStaticEntry.clone(false).appendTo(ql);
	    el.find('[name="answerText"]').text(response);
	} else {
	    el = qEntry.clone(false).appendTo(ql);
	    el.find('[name="answerText"]').val(response);
	}
	el.find('[name="questionText"]').text(q.Text);
    }
	
}

function getURLParameter(name) {
    return decodeURI(
        (RegExp(name + '=' + '(.+?)(&|$)').exec(location.search)||[,null])[1]
    );
}
