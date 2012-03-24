/* Author: David G. Andersen
 *
 * requires jquery to be loaded already
 */

var FETCH_QUIZLIST = "Fetching quiz list";
var CREATE_QUIZ = "Creating quiz";
var FETCH_QUIZ = "Fetching quiz data";
var SAVE_QUIZ = "Saving quiz";
var userid = "userid";

$(document).ready(function()
{
//    $("#qbox").keyup(doQuery);
    fetchQuizList();
    $("#create_btn").click(createQuiz);
    $("#save_btn").click(saveQuiz);
    $("#newitem").click(newItem);
    make_items_sortable();
});

function make_items_sortable() {
    $("#questionlist").sortable({
	start: function(event, ui) {
	    draggable_sibling = $(ui.item).prev();
	},
	stop: function(event, ui) {
	}
    });
}

function newItem() {
    $("#questionlist").append($("#question_input").html())
    return false;
}

function parse_question(el) {
    var q = new Object();
    q.Text = el.find('#question_text').val();
    return q;
}

function saveQuiz() {
    var q = new Object();
    q.ID = $('#edit_id').val();
    q.Title = $('#edit_name').val();
    q.Questions = new Array();
    $('#questionlist li').each(function(i, el) {
	q.Questions[i] = parse_question($(this));
    });
    status(SAVE_QUIZ);
    as_string = JSON.stringify(q);
    $.post("/qu", {q: as_string}, saveQuizDone, "json");
    return false;
}

function saveQuizDone(r) {
    if (r["status"] == "failed") {
	alert("saving changes failed!");
    }
    removeStatus(SAVE_QUIZ);
    fetchQuizList();
}

function createQuiz() {
    $("label#name_error").hide();
    
    var qname = $(document).find('input[id="name"]').val()
    var label_error = ""
    if (qname == "") {
	label_error = "Name can't be blank";
    }
    if (label_error != "") {
	$("label#name_error").text(label_error);
	$("label#name_error").show();
	$("input#name").focus();
	return false;
    }
    status(CREATE_QUIZ);
    $.post("/qc", {qname: qname }, quizCreateDone, "json");
    return false;
}

function quizCreateDone(r) {
    $("input#name").val('')
    removeStatus(CREATE_QUIZ)
    fetchQuizList();
}

function fetchQuizList() {
    status(FETCH_QUIZLIST)
    $.post("/ql", {u: userid }, gotQuizList, "json");
}
function gotQuizList(r) {
    removeStatus(FETCH_QUIZLIST)
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
    $('#quizlist').empty()
    for (var i = 0; i < ql.length; i++) {
	var editlink = "editlink_"+ql[i].ID;
	$('#quizlist').append('<li><a href="#edit" id="'+editlink+'">'+ql[i].Title+'</a> - '+ql[i].ID+' ('+ql[i].Created+')</li>');
	$('#'+editlink).click({id: ql[i].ID}, editQuiz);
    }
}

function editQuiz(event) {
    quizID = event.data.id;
    status(FETCH_QUIZ);
    $.post("/qget", {q: quizID}, editQuizGotQuizInfo, "json");
    return false;
}

function editQuizGotQuizInfo(r) {
    quiz = r['quiz'];
    $('#edit_name').val(quiz.Title);
    $('#edit_id').val(quiz.ID);
    $('#edit').show();
    $('#questionlist').empty()
    if (quiz.Questions) {
	qhtml = $('#question_input').html()
	for (var i = 0; i < quiz.Questions.length; i++) {
	    var q = quiz.Questions[i];
	    $('#questionlist').append(qhtml);
	    el = $('#questionlist li').last() // xxx - this is n^2. :(
	    el.find("#question_text").val(q.Text)
	}
    }
}

function doQuery() {
    var qdat = $(document).find('input[id="qbox"]').val();
    $.post("/q", {q: qdat }, gotResult, "json");
}

function gotResult(r) {
    $('#val').text(r['val'])
}

function status(s) {
    $('#status').text(s)
}
function removeStatus(s) {
    $('#status').text('')
}