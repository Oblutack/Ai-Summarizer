"""Documents with known facts, for the summary-quality check (run_eval.py).

Each case is a made-up document, the facts a good summary of it must keep, the mistakes that kind of document
invites (traps), and a hand-written reference summary that must score clean (test_summary_quality.py checks
that, so a broken pattern is caught before it can mislead a run). All of the documents are invented.

Several traps come from mistakes the summarizer was really seen to make: "4.1 million" turned into
"$4.1 million" when the text names no currency, "10% on the previous quarter" turned into "10% year on year",
a risk credited to a document that never mentions it, a "=====" underline under a title.
"""

import re
from dataclasses import dataclass

from summary_quality import Fact, Trap


@dataclass(frozen=True)
class Variant:
    style: str = "default"
    language: str = "English"
    word_count: int = 150


@dataclass(frozen=True)
class Case:
    id: str
    kind: str  # "single", "multi" (several documents summarized together) or "overview" (a collection)
    docs: tuple[tuple[str, str], ...]  # (name, text); one entry for a single document
    facts: tuple[Fact, ...]
    traps: tuple[Trap, ...]
    variants: tuple[Variant, ...]
    reference: str
    allowed_numbers: tuple[str, ...] = ()
    subject: str = ""  # the collection's name, for an overview

    @property
    def text(self) -> str:
        """What the document says, for checking numbers: all of the documents together."""
        return "\n\n".join(f"=== {name} ===\n{text}" for name, text in self.docs) if len(self.docs) > 1 else self.docs[0][1]


DEFAULT = Variant()
BULLETS = Variant("bullets")
BRIEF = Variant("brief")
SIMPLE = Variant("simple")
TAKEAWAYS = Variant("takeaways")
SPANISH = Variant("default", "Spanish")

# A month and the figure written right after it ("March: revenue 5,200"), not across "and" in a list.
_GAP = r"(?:(?!\band\b)[^.;,\n\d]){0,24}"

# A document that names euros: any other currency is a mistake. One that names none: any currency is.
OTHER_CURRENCY = Trap("wrong currency", r"[$£]|\b(?:dollars?|usd|pounds?|gbp)\b")
INVENTED_CURRENCY = Trap("invents a currency", r"[$€£]|\b(?:dollars?|euros?|usd|eur|pounds?|gbp)\b")


# ---- 1. a lease: figures, a negative condition, names ---------------------------------------------------------

LEASE = """RESIDENTIAL LEASE AGREEMENT

This agreement is made on 14 February 2025 between Marta Kovač (the landlord) and Daniel Reyes (the tenant) for the apartment at 12 Harbour Street, second floor.

1. Term. The lease starts on 1 March 2025 and runs for twelve months. After that it continues month to month unless either party ends it.

2. Rent. The tenant pays 950 euros per month, due on the first day of each month by bank transfer. Rent is not increased during the first twelve months.

3. Deposit. The tenant pays a deposit of 1,900 euros before moving in. The deposit is returned within 30 days after the lease ends, minus the cost of any damage beyond normal wear.

4. Utilities. Water and heating are included in the rent. The tenant pays electricity and internet directly to the providers.

5. Pets. Pets are allowed only with the landlord's written permission. Smoking is not allowed inside the apartment.

6. Ending the lease. Either party may end the lease with three months' written notice. The tenant must leave the apartment clean and return all keys on the last day.

7. Repairs. The tenant must report defects promptly. The landlord pays for repairs of the heating, plumbing and the roof; the tenant pays for small repairs under 50 euros."""

LEASE_CASE = Case(
    id="lease",
    kind="single",
    docs=(("Lease agreement", LEASE),),
    facts=(
        Fact("rent 950", (r"\b950\b",)),
        Fact("deposit 1,900", (r"1[ ,.]?900",)),
        Fact("deposit back in 30 days", (r"\b30\b|thirty",)),
        Fact("three months' notice", (r"(three|3)[ -]months?",), neutral=False),
        Fact("pets need written permission", (r"pets?.{0,80}(written|permission|consent|approv)", r"(written|permission|consent|approv).{0,80}pets?"), neutral=False),
        Fact("starts 1 March 2025", (r"1 march|march 1\b|2025-03-01|01/03/2025",), neutral=False),
        Fact("landlord Marta Kovač", (r"marta|kova",)),
        Fact("tenant Daniel Reyes", (r"daniel|reyes",)),
    ),
    traps=(
        OTHER_CURRENCY,
        Trap("rent and deposit swapped", r"rent[^.;,\n\d]{0,40}1[ ,.]?900|deposit[^.;,\n\d]{0,40}\b950\b"),
    ),
    variants=(DEFAULT, BULLETS, SPANISH),
    reference=(
        "## Lease agreement\n"
        "- Landlord Marta Kovač rents the apartment at 12 Harbour Street to tenant Daniel Reyes from 1 March 2025 for twelve months.\n"
        "- Rent is 950 euros per month; the deposit is 1,900 euros and is returned within 30 days after the lease ends.\n"
        "- Pets are allowed only with the landlord's written permission, and either party can end the lease with three months' written notice."
    ),
)


# ---- 2. a quarterly report: a comparison period and a currency ------------------------------------------------

NORTHWIND_Q2 = """Northwind quarterly report, Q2

Revenue. Revenue in the second quarter was 4.4 million euros, an increase of 10% compared with the previous quarter. The growth came mainly from subscription renewals.

Support. Support tickets fell by a third after the offline mode launched in May. Customers who use offline mode raised far fewer connection problems.

People. The team grew to 46 employees after six new hires in engineering and support.

Risks. The main risk is rising infrastructure costs, which went up 21% over the quarter as usage increased. Management is negotiating a longer contract with the cloud provider.

Plans. Northwind will hire three more engineers and open the Lisbon office in September. A review of pricing is planned for the autumn."""

NORTHWIND_CASE = Case(
    id="northwind_q2",
    kind="single",
    docs=(("Northwind Q2 report", NORTHWIND_Q2),),
    facts=(
        Fact("revenue 4.4 million", (r"4[.,]4",)),
        Fact("up 10%", (r"\b10 ?%|ten percent|10 percent",)),
        Fact("tickets down a third", (r"third|33 ?%|\b1/3\b",), neutral=False),
        Fact("46 employees", (r"\b46\b",)),
        Fact("Lisbon office", (r"lisbon|lisboa|lissabon|lisbonne",)),
        Fact("infrastructure costs up 21%", (r"21 ?%|21 percent",)),
    ),
    traps=(
        OTHER_CURRENCY,
        Trap("year on year", r"year[- ]?(?:over|on)[- ]?year|\byoy\b|annual growth|compared (?:with|to) (?:last year|the same quarter)|\byear-over"),
        Trap("21% credited to revenue", r"revenue[^.;,\n\d]{0,40}\b21 ?%"),
        Trap("10% credited to infrastructure", r"infrastructure[^.;,\n\d]{0,40}\b10 ?%"),
    ),
    variants=(DEFAULT, BRIEF),
    allowed_numbers=("33", "1"),  # "a third" as 33%, and Q1 as the quarter before Q2, follow from the text
    reference=(
        "## Northwind Q2\n"
        "- Revenue was 4.4 million euros, up 10% on the previous quarter, and support tickets fell by a third after offline mode launched.\n"
        "- The team grew to 46 employees. The main risk is infrastructure costs, up 21%.\n"
        "- Plans: hire three engineers and open the Lisbon office in September."
    ),
)


# ---- 3. a slide deck: figures with no currency stated ---------------------------------------------------------

DECK = """Q3 Business Review

Revenue
Revenue grew 12% to 4.1 million
Two large customers paused their contracts in August

Support
Support tickets fell by a third
Offline mode is now used by 60% of customers

Risks
Infrastructure costs rose 21%
Two key engineers are leaving in November

Plan
Hire three engineers by January
Cut cloud spend by 15%
Review pricing in the autumn"""

DECK_CASE = Case(
    id="deck",
    kind="single",
    docs=(("Q3 business review", DECK),),
    facts=(
        Fact("revenue up 12%", (r"12 ?%",)),
        Fact("4.1 million", (r"4[.,]1",)),
        Fact("tickets down a third", (r"third|33 ?%",), neutral=False),
        Fact("offline mode 60%", (r"60 ?%",)),
        Fact("infrastructure up 21%", (r"21 ?%",)),
        Fact("engineers leave in November", (r"november",), neutral=False),
        Fact("cut cloud spend 15%", (r"15 ?%",)),
    ),
    traps=(INVENTED_CURRENCY,),
    variants=(DEFAULT, SIMPLE),
    allowed_numbers=("33",),  # "a third" as 33%
    reference=(
        "## Q3 business review\n"
        "- Revenue grew 12% to 4.1 million, although two large customers paused their contracts in August.\n"
        "- Support tickets fell by a third and 60% of customers now use offline mode.\n"
        "- Infrastructure costs rose 21% and two key engineers leave in November. The plan is to hire three engineers by January and cut cloud spend by 15%."
    ),
)


# ---- 4. a news article: names, places and figures -------------------------------------------------------------

NEWS = """Harbour Bridge reopens after 14 months of renovation

The Harbour Bridge reopened to traffic on 6 June after a renovation that lasted 14 months. Mayor Elena Marković cut the ribbon in front of several hundred residents and thanked the workers of Danubia Construction, who carried out the project.

The renovation cost 38 million euros, 9 million more than the original budget. Councillor Tomas Berg of the opposition called the overrun "a failure of planning" and asked for an independent audit. The mayor replied that the extra money paid for a stronger steel deck that will last at least 60 years.

The bridge is 412 metres long and carries about 22,000 vehicles a day. A new protected lane for cyclists opens on the east side, and the pavement on the west side has been widened to three metres.

During the works the city ran extra ferries across the river. The ferries will stop running on 30 June."""

NEWS_CASE = Case(
    id="news",
    kind="single",
    docs=(("Harbour Bridge reopens", NEWS),),
    facts=(
        Fact("reopened 6 June", (r"6 june|june 6\b",), neutral=False),
        Fact("14 months", (r"\b14\b|fourteen",)),
        Fact("cost 38 million", (r"\b38\b",)),
        Fact("9 million over budget", (r"\b9 million|\b9m\b|nine million",)),
        Fact("Mayor Elena Marković", (r"elena|markovi",)),
        Fact("Danubia Construction", (r"danubia",)),
        Fact("Councillor Tomas Berg", (r"berg",)),
        Fact("412 metres", (r"\b412\b",)),
        Fact("22,000 vehicles", (r"22[ ,.]?000",)),
    ),
    traps=(
        OTHER_CURRENCY,
        Trap("cost and overrun swapped", r"(?:cost|price)(?:(?!over|extra|more|above|beyond|increas)[^.;,\n\d]){0,30}\b9 million|overrun[^.;,\n\d]{0,30}\b38\b"),
        Trap("mayor and councillor swapped", r"mayor[^.;,\n\d]{0,25}\bberg\b|councillor[^.;,\n\d]{0,25}\bmarkovi"),
    ),
    variants=(DEFAULT,),
    reference=(
        "## Harbour Bridge reopens\n"
        "- The Harbour Bridge reopened on 6 June after a 14-month renovation by Danubia Construction; Mayor Elena Marković cut the ribbon.\n"
        "- It cost 38 million euros, 9 million over budget, which opposition councillor Tomas Berg criticised and wants audited.\n"
        "- The 412-metre bridge carries about 22,000 vehicles a day and now has a protected cycle lane."
    ),
)


# ---- 5. a technical text with code: a title and angle brackets ------------------------------------------------

CPP = """Dynamic array allocation: understanding the fundamentals

Fixed-size arrays must have their size known at compile time. When the size is only known while the program runs, memory has to be allocated dynamically on the heap with the new operator.

#include <iostream>

int main() {
    int n = 5;
    int* data = new int[n];   // allocate n integers on the heap
    for (int i = 0; i < n; ++i) data[i] = i * i;
    std::cout << data[4] << std::endl;
    delete[] data;            // release the memory
    return 0;
}

Every allocation made with new[] must be released with delete[]. Forgetting to do so is a memory leak, and using delete instead of delete[] on an array is undefined behaviour. Accessing the array after it has been deleted is a dangling pointer error.

Modern C++ code rarely manages arrays by hand. The standard library class std::vector owns a dynamic array, grows it automatically when elements are added, and releases the memory when it goes out of scope. Vectors are safer and just as fast in most programs, but understanding dynamic allocation explains how they work."""

CPP_CASE = Case(
    id="cpp",
    kind="single",
    docs=(("Dynamic array allocation", CPP),),
    facts=(
        Fact("allocated on the heap with new", (r"\bnew\b",), neutral=False),
        Fact("delete[] releases it", (r"delete\s*\[\s*\]",)),
        Fact("memory leak if forgotten", (r"leak",), neutral=False),
        Fact("std::vector manages it", (r"vector",)),
        Fact("heap", (r"\bheap\b",), neutral=False),
    ),
    traps=(
        Trap("heading underlined with ====", r"(?m)^\s*={3,}\s*$"),
        Trap("delete instead of delete[]", r"(?:use|call)s? `?delete`?(?! ?\[)[^.;,\n\d]{0,30}(?:to )?(?:free|release)[^.;,\n\d]{0,20}array"),
    ),
    variants=(DEFAULT, BULLETS),
    reference=(
        "## Dynamic array allocation\n"
        "- When an array's size is only known at run time, memory is allocated on the heap with new int[n] and must be released with delete[].\n"
        "- Forgetting delete[] is a memory leak, and using the array after deleting it is a dangling pointer error.\n"
        "- In modern C++ std::vector owns a dynamic array and manages it automatically, which is safer."
    ),
    allowed_numbers=("4", "5", "16"),  # the sample prints data[4], which is 16
)


# ---- 6. meeting notes: owners and deadlines -------------------------------------------------------------------

MEETING = """Project Atlas weekly sync, Tuesday 11 March

Present: Priya Nair, Tom Becker, Aisha Khan.

Decisions
- The launch moves from 28 March to 4 April because testing found 3 critical bugs.
- A budget increase of 12,000 euros was approved to cover extra testing.

Actions
- Priya Nair will send the revised budget to finance by Friday 14 March.
- Tom Becker will book the venue for the launch event by 21 March.
- Aisha Khan will fix the three critical bugs by 28 March and report back on Monday 31 March.

Risks
- The supplier of the demo hardware may deliver late. Tom will check with them on Thursday.

Next meeting: Tuesday 18 March."""

MEETING_CASE = Case(
    id="meeting",
    kind="single",
    docs=(("Atlas weekly sync", MEETING),),
    facts=(
        Fact("launch moved to 4 April", (r"4 april|april 4\b",), neutral=False),
        Fact("3 critical bugs", (r"\b3\b|three",)),
        Fact("budget 12,000", (r"12[ ,.]?000",)),
        Fact("Priya sends the budget by 14 March", (r"priya[^.\n]{0,120}14 march|14 march[^.\n]{0,120}priya|priya[^.\n]{0,120}march 14|march 14[^.\n]{0,120}priya",)),
        Fact("Tom books the venue by 21 March", (r"tom[^.\n]{0,120}21 march|21 march[^.\n]{0,120}tom|tom[^.\n]{0,120}march 21|march 21[^.\n]{0,120}tom",)),
        Fact("Aisha fixes the bugs by 28 March", (r"aisha[^.\n]{0,120}28 march|28 march[^.\n]{0,120}aisha|aisha[^.\n]{0,120}march 28|march 28[^.\n]{0,120}aisha",)),
    ),
    traps=(
        Trap("action given to the wrong person", r"priya[^.;,\n\d]{0,60}(?:venue|bugs)|tom[^.;,\n\d]{0,60}(?:budget to finance|bugs)|aisha[^.;,\n\d]{0,60}(?:venue|budget)"),
        Trap("launch date wrong", r"launch[^.;,\n\d]{0,40}(?:moved|delayed|postponed|pushed)[^.;,\n\d]{0,20}to (?:21|28|31) ?(?:march)?|launch[^.;,\n\d]{0,40}(?:to|until) (?:11|14) march"),
    ),
    variants=(TAKEAWAYS, DEFAULT),
    reference=(
        "## Key Takeaways\n"
        "1. The launch moves from 28 March to 4 April because testing found 3 critical bugs.\n"
        "2. A budget increase of 12,000 euros was approved.\n\n"
        "## Action Items\n"
        "- Priya Nair will send the revised budget to finance by Friday 14 March.\n"
        "- Tom Becker will book the venue by 21 March.\n"
        "- Aisha Khan will fix the three critical bugs by 28 March."
    ),
    allowed_numbers=("1", "2"),
)


# ---- 7. a warranty: durations and exclusions ------------------------------------------------------------------

WARRANTY = """Atlas X200 limited warranty

The compressor unit is covered for seven years from the date of purchase. The drive motor is covered for three years. The filter and the belts are wear parts and are covered for 90 days only.

The warranty does not cover water damage, damage caused by using a lubricant other than the one named in the manual, or repairs made by anyone other than an authorized service center.

To make a claim, send the receipt and the serial number to support within 30 days of discovering the defect. Replacement parts are shipped free of charge within the EU. Customers outside the EU pay the shipping costs."""

WARRANTY_CASE = Case(
    id="warranty",
    kind="single",
    docs=(("Atlas X200 warranty", WARRANTY),),
    facts=(
        Fact("compressor seven years", (r"(?:compressor|unit)[^.\n]{0,60}(?:seven|\b7\b)|(?:seven|\b7\b)[^.\n]{0,40}(?:compressor|unit)",), neutral=False),
        Fact("motor three years", (r"motor[^.\n]{0,60}(?:three|\b3\b)|(?:three|\b3\b)[^.\n]{0,40}motor",), neutral=False),
        Fact("wear parts 90 days", (r"\b90\b|ninety",)),
        Fact("water damage excluded", (r"(?:not|n't|exclud|except|no cover|without)[^.\n]{0,80}water|water[^.\n]{0,60}(?:not covered|exclud|except|not included)",), neutral=False),
        Fact("claim within 30 days", (r"\b30\b|thirty",)),
        Fact("free shipping in the EU", (r"\beu\b",)),
    ),
    traps=(
        Trap("water damage presented as covered", r"(?<!not )(?<!n't )(?<!no )\bcovers? water damage|water damage (?:is|are) covered|covered[^.;,\n\d]{0,30}including water"),
        Trap("durations swapped", r"motor[^.;,\n\d]{0,40}(?:seven|\b7\b)|(?:compressor|unit)[^.;,\n\d]{0,40}(?:three|\b3\b)\b[^.;,\n\d]{0,10}year"),
    ),
    variants=(DEFAULT, SIMPLE),
    reference=(
        "## Atlas X200 warranty\n"
        "- The compressor unit is covered for seven years, the drive motor for three years, and the filter and belts for 90 days.\n"
        "- Water damage, the wrong lubricant and unauthorized repairs are not covered.\n"
        "- Claims need the receipt and serial number within 30 days of finding the defect; parts ship free within the EU."
    ),
)


# ---- 8. a table of figures: values that belong to months ------------------------------------------------------

FINANCE = """Monthly results, first quarter 2025 (thousands of euros)

January: revenue 3,800, costs 3,100, profit 700
February: revenue 4,100, costs 3,400, profit 700
March: revenue 5,200, costs 3,900, profit 1,300

Total first-quarter revenue was 13,100 and total profit was 2,700. Revenue was highest in March, when a large one-off order from the city council was delivered. Costs rose every month, mostly because of overtime in March. Management expects April revenue to fall back to about 4,000."""

FINANCE_CASE = Case(
    id="finance",
    kind="single",
    docs=(("First quarter results", FINANCE),),
    facts=(
        Fact("March revenue 5,200", (r"march[^.]{0,80}5[ ,.]?200|5[ ,.]?200[^.]{0,60}march",)),
        Fact("March profit 1,300", (r"march[^.]{0,120}1[ ,.]?300|1[ ,.]?300[^.]{0,60}march",)),
        Fact("quarter revenue 13,100", (r"13[ ,.]?100",)),
        Fact("quarter profit 2,700", (r"2[ ,.]?700",)),
        Fact("large order from the city council", (r"council",), neutral=False),
        Fact("April expected about 4,000", (r"4[ ,.]?000",)),
    ),
    traps=(
        OTHER_CURRENCY,
        Trap("March figure given to another month", r"(?:january|february)" + _GAP + r"(?:5[ ,.]?200|1[ ,.]?300)|(?:5[ ,.]?200|1[ ,.]?300) (?:in|for|during) (?:january|february)"),
        Trap("January figure given to March", r"march" + _GAP + r"\b3[ ,.]?800\b|\b3[ ,.]?800 (?:in|for|during) march"),
    ),
    variants=(DEFAULT, BULLETS),
    allowed_numbers=("10400",),  # the three months of costs, added up
    reference=(
        "## First quarter 2025\n"
        "- Revenue was 3,800 in January, 4,100 in February and 5,200 in March, a total of 13,100 (thousands of euros).\n"
        "- Profit was 700, 700 and 1,300, a total of 2,700; March was the best month because of a large order from the city council.\n"
        "- April revenue is expected to fall back to about 4,000."
    ),
)


# ---- 9. a Spanish source: the language it is summarized in ----------------------------------------------------

SPANISH_TEXT = """La biblioteca municipal de Valdivia reabre sus puertas

La biblioteca municipal de Valdivia reabrió el 3 de septiembre tras una reforma que costó 2,4 millones de euros y duró once meses. La directora, Lucía Fernández, destacó que el edificio es ahora accesible para personas con movilidad reducida y que cuenta con una nueva sala infantil.

El fondo supera los 85.000 libros y se han incorporado 3.000 títulos nuevos durante la reforma. La biblioteca abrirá de lunes a sábado, de 9 a 21 horas, y los domingos permanecerá cerrada.

El alcalde, Joaquín Ortega, anunció que en primavera se abrirá un servicio de préstamo de ordenadores portátiles. Para obtener el carnet de socio solo se necesita el documento de identidad, y el carnet es gratuito para los menores de 16 años."""

SPANISH_CASE = Case(
    id="spanish_source",
    kind="single",
    docs=(("La biblioteca de Valdivia", SPANISH_TEXT),),
    facts=(
        Fact("Valdivia", (r"valdivia",)),
        Fact("cost 2.4 million", (r"2[.,]4",)),
        Fact("85,000 books", (r"85[ ,.]?000",)),
        Fact("director Lucía Fernández", (r"luc[ií]a|fern[aá]ndez",)),
        Fact("opens Monday to Saturday 9 to 21", (r"\b21\b|9 ?(?:a|to|-|–) ?21|9:00",)),
        Fact("free card under 16", (r"\b16\b",)),
    ),
    traps=(
        Trap("wrong currency", r"[$£]|\b(?:dollars?|d[oó]lares|usd)\b"),
        Trap("mayor and director swapped", r"(?:alcalde|mayor)[^.;,\n\d]{0,25}luc[ií]a|(?:directora?|director)[^.;,\n\d]{0,25}joaqu[ií]n"),
    ),
    variants=(DEFAULT, SPANISH),
    reference=(
        "## Valdivia library reopens\n"
        "- The Valdivia municipal library reopened on 3 September after an 11-month renovation costing 2.4 million euros.\n"
        "- Director Lucía Fernández said it is now accessible and has a new children's room; the collection exceeds 85,000 books.\n"
        "- It opens Monday to Saturday from 9 to 21, and membership cards are free for children under 16."
    ),
    allowed_numbers=("11",),
)


# ---- 10. a long report: facts spread through a document that needs several passes -----------------------------

_TOPICS = [
    "boiler", "elevator", "fire alarm", "roof drainage", "generator", "water tank", "ventilation", "emergency lighting",
    "sprinkler", "parking gate", "lightning rod", "intercom", "heat pump", "gas meter", "security camera", "air filter",
    "stairwell door", "fuel store", "cooling tower", "sewage pump", "window seal", "solar inverter", "cable duct", "bell system",
    "loading dock", "fence", "paving", "irrigation", "signage", "lift cable", "chimney", "compressor room", "water softener",
    "battery bank", "radio mast", "waste press",
]
_INTERVALS = [3, 4, 5, 6, 8, 9, 10, 12, 14, 15, 18, 20, 24, 30, 36, 7, 11, 13, 16, 21, 22, 27, 28, 33, 40, 42, 45, 48, 54, 60, 17, 19, 23, 25, 26, 32]


def _long_report() -> str:
    parts = ["Facilities maintenance handbook for the Riverside campus\n"]
    for i, (topic, months) in enumerate(zip(_TOPICS, _INTERVALS), start=1):
        parts.append(
            f"Section {i}. The {topic}.\n"
            f"The {topic} is inspected by the facilities team and the findings are recorded in the campus log. "
            f"The inspection interval for the {topic} is {months} months. Between inspections, staff report any unusual noise, "
            f"smell or vibration to the duty manager, who decides whether an earlier visit is needed. Contractors who work on the {topic} "
            "must sign in at the front desk, wear the visitor badge at all times and hand in their completed work order at the end of the visit. "
            "Records are kept for the period set out in the general retention policy. Any change to the interval must be approved in writing "
            "by the head of facilities, and the old interval stays in force until the approval is filed. The campus insurer may ask to see "
            "the log at any time, so entries must be complete, dated and signed. Where a task cannot be finished in one visit, the contractor "
            "records what remains and the duty manager books the next visit within the same week. Staff who notice a safety risk at any "
            "time must not wait for the inspection and should use the emergency procedure described in the introduction."
        )
    return "\n\n".join(parts)


LONG = _long_report()
_KEY_SECTIONS = (3, 9, 15, 22, 30, 35)


def _interval_fact(index: int) -> Fact:
    topic, months = _TOPICS[index - 1], _INTERVALS[index - 1]
    return Fact(
        f"{topic} every {months} months",
        (rf"{re.escape(topic)}[^.\n]{{0,80}}\b{months}\b|\b{months}\b[^.\n]{{0,60}}{re.escape(topic)}",),
    )


LONG_CASE = Case(
    id="long_report",
    kind="single",
    docs=(("Facilities maintenance handbook", LONG),),
    facts=tuple(_interval_fact(i) for i in _KEY_SECTIONS),
    traps=(),
    variants=(Variant("default", "English", 300),),
    reference="\n".join(
        ["## Facilities handbook"] + [f"- The {_TOPICS[i - 1]} is inspected every {_INTERVALS[i - 1]} months." for i in _KEY_SECTIONS]
    ),
    allowed_numbers=tuple(str(i) for i in range(1, 37)),
)


# ---- 11. several documents together: figures that belong to one of them --------------------------------------

INVOICE = """Invoice 2025-117 from Harbour Print Ltd to Northwind

Brochures: 800 euros. Posters: 440 euros. Total due: 1,240 euros, payable by 30 April. Late payments are charged 2% a month."""

CONTRACT = """Service contract between Northwind and Brightline Cleaning

Brightline Cleaning will clean the Northwind offices three times a week for a monthly fee of 650 euros. The contract runs for 12 months from 1 May. Either side may cancel with 60 days' notice."""

EMAIL = """From: Sofia Lindqvist
To: All staff
Subject: Office move

The office move is on Saturday 17 May. The new address is 8 Quay Road. The internet will be installed on Wednesday 14 May, so please do not expect it to work before then. Boxes will be delivered on 12 May."""

MULTI_CASE = Case(
    id="multi",
    kind="multi",
    docs=(("invoice.pdf", INVOICE), ("contract.pdf", CONTRACT), ("move-email.pdf", EMAIL)),
    facts=(
        Fact("invoice total 1,240", (r"1[ ,.]?240",)),
        Fact("invoice due 30 April", (r"30 april|april 30\b",), neutral=False),
        Fact("cleaning fee 650", (r"\b650\b",)),
        Fact("cleaning cancellation 60 days", (r"\b60\b|sixty",)),
        Fact("move on 17 May", (r"17 may|may 17\b",), neutral=False),
        Fact("new address 8 Quay Road", (r"quay road",)),
    ),
    traps=(
        OTHER_CURRENCY,
        Trap("invoice and cleaning fee swapped", r"(?:invoice|harbour print)[^.;,\n\d]{0,60}\b650\b|(?:cleaning|brightline)[^.;,\n\d]{0,60}1[ ,.]?240"),
        Trap("move date given to the invoice", r"(?:invoice|harbour print)[^.;,\n\d]{0,60}(?:17|14) may|(?:cleaning|brightline)[^.;,\n\d]{0,60}(?:30 april|17 may)"),
    ),
    variants=(DEFAULT,),
    reference=(
        "## Three documents\n"
        "- invoice.pdf: Harbour Print Ltd invoices Northwind 800 euros for brochures and 440 euros for posters, 1,240 euros in total, payable by 30 April.\n"
        "- contract.pdf: Brightline Cleaning cleans the offices three times a week for 650 euros a month; 12 months from 1 May, cancellable with 60 days' notice.\n"
        "- move-email.pdf: Sofia Lindqvist says the office move is on 17 May to 8 Quay Road, with the internet installed on 14 May."
    ),
    allowed_numbers=("2", "3", "2025", "117", "12"),
)


# ---- 12. an overview of a collection: who said what, and what they agree on -----------------------------------

Q1_SUMMARY = """# Northwind Q1 report
- Revenue: 4.0 million euros, up 5% on last year.
- Support tickets rose 10% after the March release.
- Team: 40 employees.
- Main risk: dependence on a single cloud provider.
- Plan: open an office in Lisbon."""

Q2_SUMMARY = """# Northwind Q2 report
- Revenue: 4.4 million euros, up 10% on the previous quarter.
- Support tickets fell by a third after the offline mode launch.
- Team: grew to 46 employees.
- Main risk: rising infrastructure costs.
- Plans: hire three engineers and open the Lisbon office in September."""

Q3_SUMMARY = """# Northwind Q3 report
- Revenue: 4.1 million euros, down 7% on the previous quarter because two large customers paused their contracts.
- Support tickets stayed flat.
- Two key engineers leave in November.
- Main risk: customer concentration.
- Plans: cut cloud spend by 15% and postpone the Lisbon office."""

OVERVIEW_CASE = Case(
    id="overview",
    kind="overview",
    subject="Northwind",
    docs=(("Northwind Q1 report", Q1_SUMMARY), ("Northwind Q2 report", Q2_SUMMARY), ("Northwind Q3 report", Q3_SUMMARY)),
    facts=(
        Fact("Q3 revenue fell 7%", (r"7 ?%",)),
        Fact("Lisbon office postponed in Q3", (r"postpon|delay|defer|put off",), neutral=False),
        Fact("Q3 risk is customer concentration", (r"concentration",), neutral=False),
        Fact("Q2 risk is infrastructure costs", (r"infrastructure",), neutral=False),
        Fact("Q1 risk is a single cloud provider", (r"single cloud|one cloud|cloud provider",), neutral=False),
        Fact("engineers leave in November", (r"november",), neutral=False),
    ),
    traps=(
        Trap(
            "a shared risk that is not shared",
            r"(?:all three|all documents|every (?:report|document)|each (?:report|document))[^.;,\n\d]{0,60}"
            r"(?:shar|name|cite|mention|identif|highlight|point|flag|list)[^.;\n]{0,60}"
            r"(?:single cloud|cloud provider|customer concentration|infrastructure costs)",
        ),
        Trap("year on year", r"\bq[23]\b(?:[^.\n]|(?<=\d)\.(?=\d)){0,70}(?:year[- ]?(?:over|on)[- ]?year|\byoy\b)"),
        Trap("wrong currency", r"[$£]|\b(?:dollars?|usd)\b"),
        Trap("Q3 risk credited to Q1 or Q2", r"q[12][^.;,\n\d]{0,40}customer concentration"),
    ),
    variants=(DEFAULT,),
    reference=(
        "# Northwind overview\n"
        "- Revenue rose from 4.0 to 4.4 million euros in Q2 and fell 7% to 4.1 million in Q3 after two large customers paused contracts.\n"
        "- The main risk changes every quarter: a single cloud provider in Q1, infrastructure costs in Q2, customer concentration in Q3.\n"
        "- The Lisbon office is planned in Q1 and Q2 and postponed in Q3; two key engineers leave in November."
    ),
    allowed_numbers=("1", "2", "3"),
)


CASES: tuple[Case, ...] = (
    LEASE_CASE, NORTHWIND_CASE, DECK_CASE, NEWS_CASE, CPP_CASE, MEETING_CASE, WARRANTY_CASE,
    FINANCE_CASE, SPANISH_CASE, LONG_CASE, MULTI_CASE, OVERVIEW_CASE,
)  # fmt: skip
