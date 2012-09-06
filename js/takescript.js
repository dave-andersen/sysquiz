/* Author:  David G. Andersen
* 
* requires jquery and jquery-ui to be loaded already
*/

"use strict";

/* namespace for this app */
var sysquiz = {};

$(document).ready(function() {
    sysquiz.takeQuiz();
});

sysquiz.takeQuiz = function() {
    var quizRecordID = getURLParameter("q");
    $.post("/take/qget", {qr: quizRecordID}, sysquiz.takeGotQuizData, "json");
}

sysquiz.takeGotQuizData = function(r) {
    if (r['error']) {
	var e = r['error'];
	alert("Could not load quiz: " +e);
	return;
    }

    var quiz = r['quiz'];

    $('#quizName').text(quiz.Title + " (" + quiz.Questions.length + " questions)");

    var ql = $('#questionList');
    var qEntry = $('#widgets .questionInput');
    ql.empty();
    for (var i = 0; i < quiz.Questions.length; i++) {
	var q = quiz.Questions[i];
	var el = qEntry.clone(false).appendTo(ql);
	el.find(".questionText").text(q.Text);
    }
	
}

function getURLParameter(name) {
    return decodeURI(
        (RegExp(name + '=' + '(.+?)(&|$)').exec(location.search)||[,null])[1]
    );
}
