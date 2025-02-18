var storyList = document.getElementById('story_list');
var newStoryText = document.getElementById('new_story_text');
var newStoryButton = document.getElementById('new_story_button');
var newStoryTitle = document.getElementById('new_story_title');
var newStoryLink = document.getElementById('new_story_link');

document.body.onload = function (evt) {
    getStoryList(displayStoryList);
};

newStoryButton.onclick = function (evt) {
    let data = {
        content: newStoryText.value,
        title: newStoryTitle.value,
        link: newStoryLink.value
    };

    newStoryText.value = '';
    newStoryTitle.value = '';
    newStoryLink.value = '';

    fetch('/create_story', {
        method: 'POST', // or 'PUT'
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(data),
    }).then((response) => response.json())
        .then((data) => {
            getStoryList(displayStoryList);
        })
        .catch((error) => {
            console.error('Error:', error);
        });
};

storyList.onchange = function (evt) {
    if (evt.target.className.includes('count_spinner')) {
        let storyId = parseInt(evt.target.getAttribute('story_id'));
        let story = storiesById[storyId];
        story.countdown = parseInt(evt.target.value);
        updateStoryCounts(story, () => { });
    }
};

function retokenizeStory(story) {
    fetch('/retokenize_story', {
        method: 'POST', // or 'PUT'
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({ id: story.id }),
    }).then((response) => response.json())
        .then((data) => {
            getStoryList(displayStoryList);
        })
        .catch((error) => {
            console.error('Error retokenizing:', error);
        });
}


var storiesById = {};

function displayStoryList(stories) {
    stories.sort((a, b) => {
        return b.date_added - a.date_added
    });

    function storyRow(s) {
        return `<tr>
            <td>
               <input story_id="${s.id}" type="number" class="count_spinner" min="0" max="9" steps="1" value="${s.countdown}">
            </td>
            <td>
                <span title="number of times this story has been read">${s.read_count}</span>
            </td>
            <td><a class="story_title" story_id="${s.id}" href="/story.html?storyId=${s.id}">${s.title}</a></td>
            </tr>`;
    }


    let html = `<table class="story_table">
        <tr>
            <th>TODO</th>
            <th title="number of times this story has been read">Read count</th>
            <th>Title</th>
        </tr>`;

    storiesById = {};

    for (let s of stories) {
        storiesById[s.id] = s;
        html += storyRow(s);
    }

    storyList.innerHTML = html + '</table>';
};