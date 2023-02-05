# japanese_vocab

## TODO

- Map each story token to a dictionary entry. (map to specific meaning?)
X When clicking/hovering on each word in a story, display the definition and kanji.
- When baking definitions and kanji in tokens, the spelling / reading that is used in the token should be top (display others in smaller text below).
    - distinguish between readings/spellings/definitions that user has encountered from others

- When encountering a new compound word, should learn the parts as well.

- Track encountered words / kanji. (sqlite?)
- Drilling for words / kanji. Filter drill sets for encountered, for specific stories, for common features (e.g. 'godan verbs ending in つ')

- Add pitch info to entries in mongo.
- Readings should include spaces at border between kanji: e.g. 最近稼働 is given reading "さい きん か どう". Unfortunately, this info is not in the entries, so would have to infer from possible readings of the kanji. In some cases this is not fully determinable: e.g. for kanji spelling AB, might have possible readings "xy z" but also "x yz". (maybe just display such cases with special highlight, e.g. "xyz" in red indicates that it should be split but the split point is ambiguous)
- Readings should display pitch in style of https://www.gavo.t.u-tokyo.ac.jp/ojad/eng/pages/home
- Use priority to star the preferred spellings / readings.
- Display "other forms". Can we get the frequency of use for various forms from kanshudo?
- Mark old readings/spellings.
- Related words, kanji, info:
    x Show all component kanji
    - Mark ellided -i and -u sounds. (What about cases like 室 where the shi is part of the preceding syllabal?)
    - All forms of a verb, with pitch
    - Corresponding verb/noun (e.g. 読む / 読み)
    - Words with similar definitions that you've already encountered.
    - Find all homonyms with same pitch.
    - Find all homonyms with different pitch.
    - Find all homonyms from all possible verb forms


x Result entries should be sorted on server in order of fewest characters in spelling and/or reading