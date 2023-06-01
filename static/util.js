function splitOnHighPitch(str, pitch) {
    let [downPitch, upPitch] = pitch;
    //console.log(`downpitch ${downPitch}, up pitch ${upPitch}`);

    if (downPitch === 0) {
        return ['', '', str];
    }
    let mora = [];
    let s = new Set(['ゅ', 'ょ', 'ゃ', 'ャ', 'ュ', 'ョ']);
    let chars = str.split('');
    for (let i = 0; i < chars.length; i++) {
        if (s.has(chars[i + 1])) {
            mora.push(chars[i] + chars[i + 1]);
            i++;
        } else {
            mora.push(chars[i]);
        }
    }
    return [
        mora.slice(0, downPitch - 1).join(''),
        mora.slice(downPitch - 1, downPitch).join(''),
        mora.slice(downPitch).join('')
    ];
}

function displayKanji(kanji, word) {
    html = '';

    if (!kanji || kanji.length === 0) {
        kanjiResultsDiv.innerHTML = '';
        return;
    }

    for (let ch of new Set(word.split(''))) {
        for (let k of kanji) {
            if (k.literal === ch) {
                for (let group of k.readingmeaning.group) {
                    onyomi = group.reading.filter(x => x.type === 'ja_on').map(x => `<span class="kanji_reading">${x.value}</span>`);
                    kunyomi = group.reading.filter(x => x.type === 'ja_kun').map(x => `<span class="kanji_reading">${x.value}</span>`);

                    var meanings = group.meaning.filter(x => !x.language).map(x => x.value);

                    var misc = '';
                    if (k.misc.stroke_count) {
                        misc += `<span class="strokes">strokes: ${k.misc.stroke_count}</span>`;
                    }
                    if (k.misc.frequency) {
                        misc += `<span class="frequency">frequency: ${k.misc.frequency}</span>`;
                    }

                    html += `<div class="kanji">
                            <div>
                            <span class="literal">${k.literal}</span>
                            <div><span class="onyomi_readings">${onyomi.join('')}</span></div>
                            <div><span class="kunyomi_readings">${kunyomi.join('')}</span></div>
                            </div>
                            <div class="kanji_meanings">${meanings.join(';  &nbsp;&nbsp;')}</div>
                            <div class="kanji_misc">${misc}</div>
                            </div>`;
                }
            }
        }
    }



    kanjiResultsDiv.innerHTML = html;
}

function getKanji(str) {
    fetch('/kanji', {
        method: 'POST', // or 'PUT'
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(str),
    }).then((response) => response.json()
    ).then((data) => {
        displayKanji(data.kanji, str);
    }).catch((error) => {
        console.error('Error:', error);
    });
}

function updateWord(word) {
    let data = {...word};
    delete data.definitions;
    fetch('/update_word', {
        method: 'POST', // or 'PUT'
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(data),
    }).then((response) => response.json()
    ).then((data) => {
        //console.log(data);
    }).catch((error) => {
        console.error('Error:', error);
    });
}


function shuffle(array) {
    if (array.length < 2) {
        return;
    }

    let idx = array.length;
    while (idx != 0) {
        // Pick a remaining element.
        let randIdx = Math.floor(Math.random() * idx);
        idx--;

        // And swap it with the current element.
        [array[idx], array[randIdx]] = [array[randIdx], array[idx]];
    }
}


function displayEntry(entry) {
    let readings = '';
    for (var r of entry.readings || []) {
        if (r.pitch) {
            let pitch = r.pitch.split(',').map(x => parseInt(x));
            let parts = splitOnHighPitch(r.reading, pitch);
            readings += `<span class="reading">${parts[0]}<span class="high_pitch">${parts[1]}</span>${parts[2]}</span>`;
        } else {
            readings += `<span class="reading unknown_pitch">${r.reading}﹖</span>`;
        }
    }

    let kenjiSpellings = '';
    for (var k of entry.kanji_spellings || []) {
        kenjiSpellings += `<span class="kanji_spelling">${k.kanji_spelling}</span>`;
    }

    let senses = '';
    for (var s of entry.senses || []) {
        let pos = s.parts_of_speech.map(x => `<span class="pos">${x}</span>`);
        senses += `<span class="sense">
            <span>${pos.join(' ')}</span>
            <span class="glosses">${s.glosses.map(x => x.value).join('; &nbsp;&nbsp;')}</span>
        </span>`;
    }
    
    return `<div class="entry">
                <div class="word">
                    <div class="readings">${readings}</div>
                    <div class="kanji_spellings">${kenjiSpellings}</div>
                    <div class="senses">${senses}</div>
                </div>
            </div>`;
}

function updateStory(story, refreshList) {
    let temp = { ...story };
    delete temp.content;
    delete temp.tokens;
    delete temp.link;
    delete temp.title;
    delete temp.words;
    fetch(`/update_story`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(temp),
    }).then((response) => response.json())
        .then((data) => {
            console.log(`Success update_story:`, data);
            if (refreshList) {
                getStoryList();
            }
        })
        .catch((error) => {
            console.error('Error marking story:', error);
        });
}

function getStoryList() {
    fetch('/stories_list', {
        method: 'GET', // or 'PUT'
        headers: {
            'Content-Type': 'application/json',
        }
    }).then((response) => response.json())
        .then((data) => {
            console.log('Stories list success:', data);
            updateStoryList(data);
        })
        .catch((error) => {
            console.error('Error:', error);
        });
}


function timeSince(date) {

    var seconds = Math.floor((new Date() - date) / 1000);
    var interval = seconds / 86400;
    return Math.floor(interval);
    

    // var interval = seconds / 31536000;
    // if (interval > 1) {
    //     return Math.floor(interval) + " years ago";
    // }
    // interval = seconds / 2592000;
    // if (interval > 1) {
    //     return Math.floor(interval) + " months ago";
    // }
    // interval = seconds / 86400;
    // if (interval > 1) {
    //     return Math.floor(interval) + " days ago";
    // }
    // interval = seconds / 3600;
    // if (interval > 1) {
    //     return Math.floor(interval) + " hours ago";
    // }
    // interval = seconds / 60;
    // if (interval > 1) {
    //     return Math.floor(interval) + " minutes ago";
    // }
    // return Math.floor(seconds) + " seconds ago";
}

const DRILL_ALL_IN_PROGRESS = -1;

var storiesById = {};

function updateStoryList(stories) {
    stories.sort((a, b) => {
        let diff = b.rank - a.rank;
        if (diff === 0) {
            return a.date_last_read - b.date_last_read
        }
        return diff;
    });

    storiesById = {};

    function storyRow(s) {
        return `<tr>
            <td><a story_id="${s.id}" href="/words.html?storyId=${s.id}">words</a></td>
            <td><a story_id="${s.id}" action="dec_rank" href="#">-</a>
                <span>${s.rank}</span>
                <a story_id="${s.id}" action="inc_rank" href="#">+</a>
            </td>
            <td><span>${s.read_count}</span></td>
            <td><span>${timeSince(s.date_last_read * 1000)}</span></td>
            <td><span>${timeSince(s.date_added * 1000)}</span></td>
            <td><a class="story_title rank${s.rank}" story_id="${s.id}" href="/story.html?storyId=${s.id}">${s.title}</a></td>
            </tr>`;

        //<td><a action="retokenize" story_id="${s.id}" href="#">retokenize</a></td>
    }

    for (let s of stories) {
        storiesById[s.id] = s;
    }

    let html = `<table class="story_table">
            <tr>
            <th>Drill words</th>
            <th>Rank</th>
            <th>Read count</th>
            <th>Days ago last read</th>
            <th>Days ago created</th>
            <th></th>
            </tr>
            <tr>
                <td class="story_table_section" colspan="6">Stories&nbsp;&nbsp; 
                    <a action="drill_in_progress" href="/words.html?storyId=${DRILL_ALL_IN_PROGRESS}">drill all words of top rank stories</a>
                </td>
            </tr>`;
    for (let s of stories) {
        
        html += storyRow(s);
        
    }
    storyList.innerHTML = html + '</table>';
};
