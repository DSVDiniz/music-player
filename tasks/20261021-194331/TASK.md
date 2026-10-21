# RANDOM_NO_REPEAT previous/next song bug

- STATUS: OPEN
- PRIORITY: 100
- TAGS: bug

While on RANDOM_NO_REPEAT, press PREVIOUS_SONG and then NEXT_SONG. The next song isn't the song you were listening to before, it's a new song.
This is the expected behaviour:
song A -> song B -> song C (current)
PREVIOUS_SONG
song A -> song B (current) -> song C
NEXT_SONG
song A -> song B -> song C (current)

This is the current behaviour:
song A -> song B -> song C (current)
PREVIOUS_SONG
song A -> song B (current) -> song C
NEXT_SONG
song A -> song B -> song D (current)
