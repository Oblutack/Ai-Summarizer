"""Harder additions: documents on nearby topics, and documents in other languages."""

DISTRACTORS = {
    "hotel": """Seaview Hotel: Guest Information

Check-in is from 15:00 and check-out is by 11:00. Late check-out until 14:00 costs 25 euros. Breakfast is served from 7:00 to 10:30 and costs 18 euros per person.

Pets. Small dogs up to ten kilograms are welcome for a fee of 12 euros per night. Cats are not accepted because of allergies among our staff.

Cancellation. Bookings can be cancelled free of charge up to 48 hours before arrival. After that the first night is charged.

Parking is available in the garage for 14 euros per day. The garage is closed between midnight and 5:00.

Wi-Fi is free throughout the building. The fitness room is open 24 hours with your room key.""",
    "washer": """Aquaclean W7 Washing Machine: Owner Manual

Warranty. The machine is guaranteed for five years on the motor and for two years on all other parts. Damage from limescale is not covered; use a descaler every three months in hard-water areas.

Loading. Maximum load is 8 kilograms of cotton. Do not overfill the drum, because the machine will vibrate and walk across the floor. Close the door firmly until it clicks.

Programmes. The eco programme at 40 degrees takes three hours and ten minutes and uses the least electricity. The quick wash takes 20 minutes and is for lightly soiled clothes.

Cleaning. Clean the detergent drawer monthly and wipe the door seal after each wash to prevent mould. If the machine smells, run an empty hot cycle at 90 degrees with a cup of vinegar.

Error codes. E18 means the water inlet is blocked; check that the tap is open and clean the filter at the back.""",
    "bank": """Everyday Account: Fees and Charges

Account fee. The monthly fee is 4.50 euros and is waived if at least 1,000 euros arrives in the account each month.

Cards. The debit card is free. Replacing a lost card costs 10 euros. Withdrawals at our own machines are free; at other banks machines the fee is 2 euros each. Abroad, a currency fee of 1.5 percent applies to card payments outside the euro area.

Transfers. Domestic transfers are free. International transfers cost 8 euros plus any fee charged by the receiving bank. Instant transfers cost 0.50 euros each.

Overdraft. The interest rate on an arranged overdraft is 11.9 percent a year. An unarranged overdraft costs an extra 5 euros per month.

To report a stolen card call our 24-hour line immediately and the card will be blocked within minutes.""",
    "vaccine": """Seasonal Flu Vaccine: Information for Patients

Who should be vaccinated. Everyone over 65, pregnant women, people with chronic conditions such as asthma or diabetes, and healthcare workers are advised to receive the vaccine every autumn.

How it works. The vaccine contains inactivated virus and cannot cause influenza. Protection builds up within about two weeks and lasts for the season. Effectiveness is typically between 40 and 60 percent.

Side effects. A sore arm, mild fever and tiredness for a day or two are common. Serious allergic reactions are very rare. Tell the nurse if you have ever had a severe reaction to eggs or to a previous vaccine.

When to postpone. If you have a fever above 38 degrees on the day, wait until you are well. A mild cold is not a reason to delay.

The vaccine is free for people in the recommended groups and costs 22 euros for everyone else.""",
    "bread": """Weekend Bread: A Beginner Guide

Ingredients. 500 grams of strong flour, 10 grams of salt, 7 grams of dried yeast and 330 millilitres of lukewarm water.

Method. Mix everything and knead for ten minutes until smooth. Let the dough rise in a covered bowl for one hour, until doubled. Shape into a loaf and rise again for 40 minutes.

Baking. Heat the oven to 230 degrees with a tray of water at the bottom for steam. Bake for 30 minutes, then lower the heat to 200 degrees for another ten. The loaf is done when it sounds hollow when tapped underneath.

Troubleshooting. A dense loaf usually means the dough did not rise long enough or the yeast was old. A pale crust means the oven was not hot enough.

Store the bread in a paper bag; it keeps for two days, and slices can be frozen for a month.""",
    "cycling": """Cycling Route: Along the River Mara

The route runs 64 kilometres from the old bridge in Tolar to the sea at Port Mara and is almost completely flat. Most cyclists need five to six hours. The surface is paved except for three kilometres of gravel near the village of Esk.

Starting. Trains to Tolar run every hour and take bicycles free of charge outside the rush hours of 7:00 to 9:00 and 16:00 to 18:00.

Services. There are bike repair stations at kilometre 20 and kilometre 45. Cafes are found in Esk and Doran. There is no shop between Doran and the coast, so carry spare water.

Safety. Cyclists share the path with walkers; ring your bell when passing. A helmet is recommended but not required. In summer the headwind from the sea can be strong in the afternoon, so many riders start at the coast and ride inland.

Bike hire is available at both ends for 15 euros a day.""",
    "sick": """Sick Leave Policy

Reporting. If you are unwell, tell your manager by 9:30 on the first day, by phone or message. Email alone is not enough.

Certificates. A doctor certificate is required from the fourth day of absence. For absences of up to three days, a self-declaration form is sufficient, up to three times per year.

Pay. The company pays full salary for the first 30 days of illness in a year, then 70 percent for up to 60 further days. After that, the statutory sickness benefit applies.

Returning. After an absence longer than four weeks, a meeting with HR is held to plan a gradual return, if wanted.

Working from home while ill is not expected. Contagious diseases such as flu must be reported so that colleagues can be informed in a suitable way.""",
    "scooter": """Moto Lira 125 Scooter: Rider Handbook

Fuel. The tank holds 6.5 litres of unleaded petrol, enough for about 200 kilometres. Use petrol with octane rating 95 or higher.

Oil. Change the engine oil every 4,000 kilometres with 10W-40 oil; the engine takes 0.9 litres. The belt drive and rollers should be inspected every 8,000 kilometres.

Tyres. Front pressure is 1.9 bar and rear pressure 2.2 bar. With a passenger, add 0.2 bar to the rear tyre.

Brakes. Both brakes are hydraulic. Check the fluid level before long trips. Pads should be replaced when the groove marks are no longer visible.

Winter storage. Disconnect the battery, add fuel stabiliser and store the scooter dry. Run the engine for ten minutes once a month. Never start the engine in a closed garage.

The helmet must be fastened whenever the scooter moves; the law requires it.""",
}

FOREIGN = {
    "es_lease": """Contrato de alquiler, Calle del Mar 7

El arrendador alquila al inquilino el piso tercero de la Calle del Mar 7 por un plazo de doce meses. La renta mensual es de 700 euros y debe pagarse antes del día cinco de cada mes. La fianza es de una mensualidad y se devuelve en un plazo de treinta días tras la entrega de las llaves.

Se permiten mascotas pequeñas, como gatos, siempre que no causen molestias a los vecinos. Está prohibido fumar en las zonas comunes.

El agua y la calefacción no están incluidas; el inquilino contrata la electricidad y el gas a su nombre. Las reparaciones menores corren a cargo del inquilino.

Para terminar el contrato antes de tiempo, el inquilino debe avisar con dos meses de antelación.""",
    "de_soup": """Kartoffelsuppe nach Omas Art

Zutaten: ein Kilogramm mehligkochende Kartoffeln, zwei Karotten, eine Stange Lauch, eine Zwiebel, ein Liter Gemüsebrühe und etwas Majoran.

Zubereitung. Das Gemüse in Würfel schneiden und in etwas Butter anbraten. Mit der Brühe aufgießen und 40 Minuten bei mittlerer Hitze kochen, bis die Kartoffeln weich sind. Die Hälfte der Suppe pürieren, damit sie schön cremig wird.

Zum Schluss mit Salz, Pfeffer und Majoran abschmecken. Wer mag, gibt Würstchen dazu. Die Suppe schmeckt am nächsten Tag noch besser und lässt sich gut einfrieren.

Tipp: Ein Schuss Essig macht die Suppe frischer.""",
}

# (question, document, phrase, kind). kind: hard = a nearby document could fool the search,
# lang = question and document in the same other language, xl = question and document in different languages.
QUESTIONS = [
    ("Can I bring my cat to the hotel?", "hotel", "Cats are not accepted", "hard"),
    ("How do I get rid of a bad smell in the washing machine?", "washer", "run an empty hot cycle", "hard"),
    ("What does it cost to take money out abroad with my card?", "bank", "currency fee of 1.5 percent", "hard"),
    ("Do people with egg allergies need to be careful with the flu shot?", "vaccine", "severe reaction to eggs", "hard"),
    ("Why is my homemade loaf so heavy?", "bread", "A dense loaf usually means", "hard"),
    ("Can I take my bike on the train to the starting point?", "cycling", "take bicycles free of charge", "hard"),
    ("How many days can I stay home sick without seeing a doctor?", "sick", "self-declaration form", "hard"),
    ("What pressure should the scooter wheels have?", "scooter", "Front pressure is 1.9 bar", "hard"),
    ("Does my employer pay me when I am off ill for two months?", "sick", "full salary for the first 30 days", "hard"),
    ("What is the interest charged if I spend more than I have?", "bank", "11.9 percent", "hard"),
    ("Is breakfast included at the seaside accommodation?", "hotel", "Breakfast is served", "hard"),
    ("¿Cuánto es el alquiler mensual del piso de la Calle del Mar?", "es_lease", "700 euros", "lang"),
    ("¿Puedo tener un gato en el piso?", "es_lease", "Se permiten mascotas pequeñas", "lang"),
    ("¿Con cuánta antelación debo avisar para terminar el contrato?", "es_lease", "dos meses de antelación", "lang"),
    ("Wie lange muss die Kartoffelsuppe kochen?", "de_soup", "40 Minuten", "lang"),
    ("Kann man die Suppe einfrieren?", "de_soup", "lässt sich gut einfrieren", "lang"),
    ("How long does the potato soup have to simmer?", "de_soup", "40 Minuten", "xl"),
    ("Are pets allowed in the Spanish apartment?", "es_lease", "Se permiten mascotas pequeñas", "xl"),
]

# Questions the library cannot answer, to see how high unrelated questions score.
UNANSWERABLE = [
    "What is the capital of Australia?",
    "How do I change the battery in an electric car?",
    "Who won the football world cup in 2014?",
    "What are the symptoms of a broken ankle?",
    "How do I set up a home wireless router?",
    "What is the boiling point of water at altitude?",
    "Explain quantum chromodynamics",
    "How do I file my income tax return?",
    "What is the best way to learn the guitar?",
    "Who painted the Mona Lisa?",
    "How many moons does Jupiter have?",
    "Tell me about the French revolution",
    "How do I train a puppy to sit?",
    "What time does the library open on Sundays?",
    "Wie funktioniert ein Elektromotor?",
]
