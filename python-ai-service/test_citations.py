import pytest

from citations import clean_citations, strip_citations


def test_valid_markers_are_kept_and_reported_in_order_of_first_use():
    answer, cited = clean_citations("Warranty is 7 years [2]. Filters change every 90 days [1][2].", {1, 2, 3})
    assert answer == "Warranty is 7 years [2]. Filters change every 90 days [1][2]."
    assert cited == [2, 1]


def test_numbers_that_match_no_excerpt_are_removed():
    answer, cited = clean_citations("It is seven years [9]. Also true [1].", {1, 2})
    assert answer == "It is seven years. Also true [1]."
    assert cited == [1]


def test_comma_lists_become_separate_markers_and_drop_invalid_members():
    answer, cited = clean_citations("Both sources agree [1, 3, 8].", {1, 3})
    assert answer == "Both sources agree [1][3]."
    assert cited == [1, 3]


def test_no_markers_means_no_sources():
    assert clean_citations("I could not find that in the document.", {1, 2}) == (
        "I could not find that in the document.",
        [],
    )


def test_a_repeated_number_is_reported_once():
    _, cited = clean_citations("A [1]. B [1]. C [1][1].", {1})
    assert cited == [1]


@pytest.mark.parametrize("text", ["array[0] stays as it is? no: [a] and [] too", "version [1.2] and [x1]"])
def test_other_square_brackets_are_left_alone(text):
    answer, cited = clean_citations(text, {1})
    # "[0]" in the first sample is a number, but 0 is not an excerpt, so it is dropped like any invalid one.
    assert cited == []
    assert "[a]" in answer or "[1.2]" in answer


def test_history_loses_its_markers_so_old_numbers_cannot_mislead():
    assert strip_citations("It lasts seven years [2]. Filters too [1][3].") == "It lasts seven years. Filters too."
    assert strip_citations("No markers here.") == "No markers here."


@pytest.mark.parametrize(
    ("written", "expected"),
    [
        ("Three years【2】.", "Three years[2]."),
        ("Three years 【2】.", "Three years [2]."),
        ("Both【1, 2】 agree.", "Both[1][2] agree."),
        ("Sourced【2†source】 text.", "Sourced[2] text."),
        ("Mixed [1] and 【2†L4-L9】.", "Mixed [1] and [2]."),
    ],
)
def test_the_models_own_citation_styles_are_normalized(written, expected):
    answer, cited = clean_citations(written, {1, 2})
    assert answer == expected
    assert cited


def test_invalid_numbers_in_the_models_style_are_dropped_too():
    answer, cited = clean_citations("Claim【9】 stands.", {1, 2})
    assert answer == "Claim stands." and cited == []


def test_history_markers_in_the_models_style_are_stripped():
    assert strip_citations("Three years 【2】. And more【1†source】.") == "Three years. And more."
