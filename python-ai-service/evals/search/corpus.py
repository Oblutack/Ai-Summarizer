"""A small invented library and a question set for measuring document search.

Each question names the document it should be answered from and a phrase that must appear in the
passage that answers it. kind: "kw" = the question shares words with the answer, "para" = same
meaning in other words, "ind" = a conceptual question where the document never states the topic."""

DOCS = {
    "lease": """Residential Lease Agreement, Harbour Street 14

The landlord lets the second floor apartment at Harbour Street 14 to the tenant for a fixed term of twelve months, beginning on 1 March. The monthly rent is 950 euros and is due on the first day of every month by bank transfer. A security deposit of two months' rent is held in a separate account and returned within 30 days after the keys are handed back, minus the cost of any damage beyond normal wear.

Utilities. Heating and water are included in the rent. Electricity and internet are contracted by the tenant directly. The landlord pays the building insurance; the tenant should take out contents insurance.

Pets and smoking. Cats and dogs are not allowed without written permission from the landlord. Smoking is forbidden inside the apartment and on the balcony.

Ending the agreement. Either side may end the lease early with three months' written notice. If the tenant leaves before the end of the term without notice, the landlord may keep the deposit. Repairs of up to 100 euros per incident, such as a dripping tap or a broken light switch, are paid by the tenant.

Visitors may stay up to four weeks. Subletting any part of the apartment requires the landlord's written consent.""",
    "laptop": """Nimbus 14 Laptop: Care and Warranty Guide

Battery. The Nimbus 14 has a 56 Wh lithium battery that lasts about eleven hours of light use. To keep it healthy, avoid leaving it at 0 percent for days and avoid storing it fully charged in a hot car. The battery is rated for 800 full charge cycles before it drops below 80 percent of its original capacity.

Charging. Use only the supplied 65 W USB-C adapter. A full charge from empty takes roughly two hours. The charging light turns amber while charging and white when full.

Warranty. The laptop is covered for 24 months from the date of purchase against manufacturing defects. Accidental damage, such as a cracked screen or liquid spilled on the keyboard, is not covered. The battery has a separate warranty of 12 months. Keep your original receipt: claims are refused without proof of purchase.

Troubleshooting. If the laptop will not start, hold the power button for 15 seconds to force a restart. If the fan runs loudly, check that the vents on the underside are not blocked by a blanket or cushion. Do not open the case yourself; this voids the warranty.

Cleaning. Wipe the screen with a dry microfibre cloth. Never spray liquid directly onto the laptop.""",
    "travel": """Company Travel and Expense Policy

Booking. All flights and hotels must be booked through the company travel desk at least 14 days before departure. Economy class is required for flights under six hours. Business class is allowed only for flights longer than six hours and needs approval from a director.

Hotels. The nightly limit is 140 euros in most cities and 190 euros in London, Paris and Zurich. Minibar charges and in-room films are never reimbursed.

Meals. Employees receive a daily allowance of 45 euros for meals while travelling. Alcohol is not reimbursed. Client dinners may exceed the allowance with the manager's approval and a list of attendees.

Getting around. Use public transport or the airport train where possible. Taxis are reimbursed when public transport is unavailable or after 10 pm. Car rental needs approval, and fuel is reimbursed only with a receipt. Personal mileage is paid at 0.30 euros per kilometre.

Expenses. Submit your expense report within 30 days of returning. Reports without receipts for items over 25 euros will be returned. Reimbursement arrives with the next monthly salary.""",
    "medicine": """Patient Information: Cedrin 200 mg tablets (pain and fever relief)

What it is for. Cedrin relieves mild to moderate pain such as headache, toothache, period pain and muscle ache, and reduces fever.

How to take it. Adults and children over 12 years take one tablet every six hours, with a glass of water and preferably with food. Do not take more than four tablets in 24 hours. Do not use for longer than three days for fever or ten days for pain without asking a doctor.

Do not take Cedrin if you are allergic to ibuprofen or aspirin, have a stomach ulcer, severe heart or kidney problems, or are in the last three months of pregnancy. Ask a pharmacist before use if you take blood thinners.

Possible side effects. Common: heartburn, nausea and mild stomach pain. Rare: dizziness, skin rash and swelling of the face, which need immediate medical help. If you miss a dose, skip it; never take a double dose.

Storage. Keep out of the sight and reach of children. Store below 25 degrees Celsius in the original packaging. Do not use after the expiry date printed on the box.""",
    "recipes": """Grandma's Winter Kitchen

Pumpkin soup. Roast one kilogram of pumpkin with two onions and four cloves of garlic for 40 minutes at 200 degrees. Blend with a litre of vegetable stock, add a spoonful of smoked paprika, and finish with a splash of cream. Serves six and freezes well for up to three months.

Apple strudel. Roll the dough until you can read a newspaper through it. Fill with five sliced apples, 80 grams of raisins, cinnamon and breadcrumbs fried in butter. Bake for 35 minutes at 190 degrees until golden. Dust with icing sugar while still warm.

Lentil stew. Soak 300 grams of brown lentils for an hour. Simmer with carrots, celery and a bay leaf for 45 minutes. Add a spoonful of vinegar at the end, which brightens the flavour. A good meal for vegetarians; add smoked sausage for the others.

Hot spiced cider. Warm two litres of apple juice with three cinnamon sticks, six cloves and an orange cut in slices. Do not boil. Keep warm on the lowest heat for up to two hours.

Tip: Salt the stew only at the end, because lentils turn tough if salted early.""",
    "hiking": """Trail Guide: The Three Lakes Circuit

The circuit is 18 kilometres long and takes most walkers about seven hours, including breaks. The total ascent is 920 metres. Start at the Alpenrose car park, where parking costs 6 euros per day, and walk clockwise.

Difficulty. The route is graded moderate. The only demanding section is the scree slope below the Saddle Pass, where a fixed cable helps in wet conditions. Trekking poles are useful on the descent.

Water and food. There is a mountain hut at the second lake, open from 20 June to 30 September, serving soup, bread and cake. There is no drinking water between the first and the second lake, so carry at least two litres per person.

Weather and safety. Thunderstorms are common after midday in July and August; aim to be off the pass by 1 pm. Snow can remain on the pass until mid June. There is mobile coverage at the car park and at the hut, but not on the pass. In an emergency call 112.

Dogs are welcome on a lead. Camping is not permitted anywhere along the route.""",
    "parental": """Parental Leave Policy

Eligibility. All employees who have worked for the company for at least six months are eligible for parental leave, whether they are a birth parent, a partner or an adoptive parent.

Paid leave. The company pays 16 weeks of leave at full salary. Employees may take this in one block or split it into two blocks within the first twelve months after the birth or adoption. A further 10 weeks of unpaid leave can be added.

Notice. Please tell your manager and HR at least eight weeks before your intended start date. In the case of a premature birth, tell us as soon as you can.

Returning to work. Employees returning from leave may work a reduced schedule of 80 percent hours at full pay for the first four weeks. The company keeps your position or an equivalent one open. Health insurance continues during paid leave.

Fathers and partners receive the same 16 weeks as birth parents. There is a parent room on the third floor with a fridge and a rocking chair for those who need to pump milk or have a quiet moment.""",
    "car": """Maintenance Schedule: Vela Hatchback 1.5

Oil and filter. Change the engine oil and oil filter every 15,000 kilometres or once a year, whichever comes first. Use fully synthetic 5W-30 oil. The engine takes 4.2 litres including the filter.

Tyres. Check tyre pressure monthly: 2.3 bar at the front and 2.1 bar at the rear when cold. Rotate the tyres every 10,000 kilometres. Replace tyres when the tread is below 3 millimetres in winter or 1.6 millimetres by law.

Brakes. Brake fluid should be replaced every two years regardless of mileage, because it absorbs moisture. Front brake pads usually last about 40,000 kilometres, rear pads about 70,000.

Timing belt. The timing belt must be replaced at 120,000 kilometres or after eight years. A broken belt destroys the engine.

Warning lights. A red oil can symbol means stop at once and switch off the engine. An amber engine symbol means have the car checked soon. If the battery symbol stays on while driving, the alternator may be failing.

Winter. Switch to winter tyres when the temperature drops below 7 degrees. Check coolant concentration before the first frost.""",
    "conference": """Report: DataBridge 2026 Conference, Lisbon

The conference ran for three days in November with about 2,400 attendees from 61 countries. The opening keynote argued that most companies still waste half of their data work on cleaning and moving data around. Surveys cited in the talk found that analysts spend 45 percent of their week on preparation tasks.

Highlights. The most discussed session was a case study from a regional hospital group that cut missed appointments by 22 percent after sending personalised reminders. A panel on privacy warned that new regulations will require companies to delete customer records on request within 30 days.

Tools. Several vendors presented assistants that write database queries from plain questions. Attendees were sceptical about accuracy: one speaker showed that such tools gave wrong answers on roughly one query in five when the data was messy.

Logistics. The venue was easy to reach by metro, but the lunch queues were long and the wireless network failed on the second afternoon. Organisers promised refunds of the workshop fee to affected attendees.

Next year the conference moves to Vienna, in October.""",
    "school": """Greenfield Primary School: Parent Handbook

School day. Doors open at 8:15 and lessons begin at 8:30. Pupils are collected between 15:00 and 15:20. After 15:30 children are taken to the after-school club, which costs 6 euros per afternoon.

Absence. If your child is ill, please phone the school office before 9:00 on the first day. A written note is needed after three days of absence. For holidays during term time, parents must request permission four weeks in advance; it is granted only in exceptional cases.

Lunch. Hot lunches cost 3.50 euros and must be ordered by Friday for the following week. Packed lunches are welcome; please do not send nuts, as several children are allergic.

Uniform. Pupils wear a green sweatshirt with the school badge. Sports clothes are a white T-shirt and black shorts. All items should be labelled with the child's name.

Illness policy. Children with a fever or sickness must stay home for 48 hours after the last symptom. Head lice should be reported to the class teacher so that other families can check.

Parent evenings take place in November and in April.""",
}

# (question, document, phrase that the answering passage must contain, kind)
QUESTIONS = [
    # --- shared words --------------------------------------------------------------------------
    ("What is the monthly rent?", "lease", "950 euros", "kw"),
    ("How long is the warranty on the Nimbus 14?", "laptop", "24 months", "kw"),
    ("What is the daily meal allowance when travelling?", "travel", "45 euros", "kw"),
    ("How many tablets can an adult take in 24 hours?", "medicine", "four tablets", "kw"),
    ("How long do you bake the apple strudel?", "recipes", "35 minutes", "kw"),
    ("How long is the Three Lakes circuit?", "hiking", "18 kilometres", "kw"),
    ("How many weeks of paid parental leave are there?", "parental", "16 weeks", "kw"),
    ("How often should the engine oil be changed?", "car", "15,000 kilometres", "kw"),
    ("How many people attended the DataBridge conference?", "conference", "2,400 attendees", "kw"),
    ("What time do lessons begin at school?", "school", "8:30", "kw"),
    ("What is the notice period for ending the lease early?", "lease", "three months' written notice", "kw"),
    ("What size is the Nimbus 14 battery?", "laptop", "56 Wh", "kw"),
    # --- same meaning, other words ----------------------------------------------------------------
    ("How much do I have to pay each month for the flat?", "lease", "950 euros", "para"),
    ("Can I keep a cat in the apartment?", "lease", "Cats and dogs are not allowed", "para"),
    ("What happens if my computer gets dropped and the display breaks?", "laptop", "cracked screen", "para"),
    ("How long does the machine run unplugged?", "laptop", "eleven hours", "para"),
    ("Am I allowed to fly business class?", "travel", "Business class is allowed only", "para"),
    ("Will the firm pay me back for the minibar?", "travel", "Minibar charges", "para"),
    ("What can I do about a headache?", "medicine", "headache", "para"),
    ("Who should not use these pills?", "medicine", "allergic to ibuprofen", "para"),
    ("I forgot to take one, should I take two next time?", "medicine", "never take a double dose", "para"),
    ("How do I make a soup with squash?", "recipes", "Pumpkin soup", "para"),
    ("Why do my beans go hard when cooking?", "recipes", "lentils turn tough if salted early", "para"),
    ("Is it safe to hike there when thunder is forecast?", "hiking", "Thunderstorms are common", "para"),
    ("Where can I buy a snack along the trail?", "hiking", "mountain hut", "para"),
    ("Can new fathers take time off after the birth?", "parental", "Fathers and partners receive the same 16 weeks", "para"),
    ("When do I need to replace the cambelt?", "car", "120,000 kilometres or after eight years", "para"),
    ("What pressure should the wheels have?", "car", "2.3 bar", "para"),
    ("What did speakers say about AI assistants that generate SQL?", "conference", "wrong answers on roughly one query in five", "para"),
    ("What does my child wear for gym?", "school", "white T-shirt and black shorts", "para"),
    # --- conceptual: the topic is never named ------------------------------------------------------
    ("What happens to my money if I move out early?", "lease", "the landlord may keep the deposit", "ind"),
    ("What should I do if the screen stays black?", "laptop", "hold the power button for 15 seconds", "ind"),
    ("What do I need to hand in to get my money back after a trip?", "travel", "Submit your expense report within 30 days", "ind"),
    ("Is this suitable for someone with kidney trouble?", "medicine", "severe heart or kidney problems", "ind"),
    ("What can I cook for a vegetarian guest?", "recipes", "A good meal for vegetarians", "ind"),
    ("Is the walk dangerous for beginners?", "hiking", "graded moderate", "ind"),
    ("What is the easiest way to avoid a breakdown that wrecks the motor?", "car", "A broken belt destroys the engine", "ind"),
    ("What was the biggest complaint about the event?", "conference", "wireless network failed", "ind"),
    ("My son has a stomach bug, when can he come back?", "school", "stay home for 48 hours after the last symptom", "ind"),
    ("Is there somewhere to feed a baby at work?", "parental", "parent room on the third floor", "ind"),
]
