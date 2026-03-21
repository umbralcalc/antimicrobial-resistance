# AMR Stewardship Simulation: Project Plan

---

## Overview

Build a stochastic simulation of antimicrobial resistance (AMR) dynamics within NHS hospital trusts, learned from freely available UK surveillance and prescribing data, with a decision science layer to evaluate and optimise antibiotic stewardship policies.

The core question: **given the current resistance profile at a trust, what prescribing guideline minimises the expected resistant infection rate over 1–5 years?**

---

## Why This Problem

- In England, antibiotic-resistant infections increased 22.7% between FY 2019/20 and FY 2024/25.
- The EU has set 2030 targets to reduce MRSA, cephalosporin-resistant *E. coli*, and carbapenem-resistant *K. pneumoniae* — but carbapenem-resistant *K. pneumoniae* incidence has been rising, and without stronger action the EU is unlikely to meet all targets.
- An estimated 35,000+ people die annually in the EU/EEA as a direct consequence of antimicrobial-resistant infections.
- A systematic review of AMR mathematical models found that only 4 out of 38 reviewed models were fully stochastic, and only 2 attempted to validate against real data. The gap for data-driven stochastic simulation with a decision science layer is wide open.

---

## The Gap This Fills

Existing work falls into three camps, none of which do what the stochadex enables:

| Approach | Example | Limitation |
|----------|---------|------------|
| Deterministic compartmental models | Ross-Macdonald adaptations for hospital transmission | No stochastic dynamics, limited policy evaluation |
| RL on synthetic environments | `abx_amr_simulator` (March 2026) — Gymnasium-compatible with "leaky-balloon" resistance abstraction | Configurable but not fitted to real surveillance data |
| ML prediction from genomic data | AMR-MoEGA, various random forest classifiers | Predicts resistance phenotype, doesn't simulate policy outcomes |

**The stochadex differentiator:** a generalised stochastic simulation engine that learns its parameters from real UK surveillance data via simulation-based inference, then evaluates candidate prescribing policies through the decision science layer — exactly the pattern proven in the rugby substitution, fishing sustainability, COVID, and helminth projects.

---

## Phase 1: Data Ingestion

### 1.1 Antibiotic prescribing data

**Source:** OpenPrescribing.net (Bennett Institute for Applied Data Science, University of Oxford)

- RESTful API, no registration required, CSV or JSON output
- GP-level prescribing data covering the last 5 years
- Breakdowns by practice, Sub-ICB Location, chemical, or BNF section
- BNF section 5.1 (Antibacterial drugs) is the primary target
- Hospital prescribing data also available

**API example:**
```
GET /api/1.0/spending_by_org/?org_type=practice&code=0501&date=2024-01-01
```

**Key variables to extract:**
- Total items and DDDs (defined daily doses) per antibiotic class per trust/practice per month
- Broad-spectrum vs narrow-spectrum ratio
- AWaRe (Access/Watch/Reserve) category breakdown

### 1.2 Resistance surveillance data

**Source:** UKHSA Fingertips AMR Local Indicators

- 80+ indicators at CCG, Acute Trust, and GP practice level
- Resistance percentages for key pathogen–antibiotic combinations
- E. coli bacteraemia incidence and resistance profiles
- MRSA, MSSA, *C. difficile* infection rates

**Source:** ECDC Surveillance Atlas / EARS-Net

- EU/EEA-wide data for 8 bacterial species from invasive isolates (blood and CSF)
- Downloadable tables, time series, and country-level breakdowns
- Useful for cross-country validation and broader context

### 1.3 National surveillance context

**Source:** ESPAUR (English Surveillance Programme for Antimicrobial Utilisation and Resistance)

- Annual reports with detailed annexes and downloadable data tables
- Resistance data from the SGSS AMR module (January 2019 onwards)
- Pathogen–antibiotic combinations used in national burden analysis
- Progress tracking against UK National Action Plan targets

### 1.4 Initial data scope

Start narrow to prove the concept:

- **Pathogen:** *E. coli* (most commonly reported species, ~39% of EARS-Net isolates)
- **Resistance phenotype:** Third-generation cephalosporin resistance (one of the EU 2030 target combinations)
- **Geography:** 5–10 NHS Acute Trusts with good data coverage
- **Time window:** 2019–2025 (post-EUCAST standardisation in EARS-Net)

---

## Phase 2: Model Structure

### 2.1 State variables

The stochadex simulation should track, at minimum:

1. **Patient population** — stochastic admission/discharge dynamics at ward or trust level, with patient-level colonisation status
2. **Colonisation process** — patients carrying susceptible or resistant *E. coli* strains, with stochastic acquisition from community baseline and within-hospital transmission
3. **Infection process** — stochastic transition from colonisation to bloodstream infection (BSI), the clinically observable outcome
4. **Prescribing process** — antibiotic selection rates by class (cephalosporins, penicillins, fluoroquinolones, carbapenems, etc.), representing the policy lever
5. **Resistance selection** — prescribing pressure shifting the resistant/susceptible strain ratio over time, with fitness cost dynamics allowing partial reversion when pressure is removed

### 2.2 Simulation diagram

The simulation structure follows the stochadex pattern — analogous to the rugby match simulation but with epidemiological state transitions:

```
┌─────────────────────────────────────────────────────┐
│                    ENVIRONMENT                       │
│  Community resistance prevalence (exogenous input)   │
└──────────────┬──────────────────────────────────────┘
               │ admission with colonisation status
               ▼
┌─────────────────────────────────────────────────────┐
│               PATIENT POPULATION                     │
│  Stochastic admission/discharge/length-of-stay       │
│  State: [susceptible-colonised, resistant-colonised, │
│          uncolonised]                                │
└──────┬──────────────────┬───────────────────────────┘
       │                  │
       ▼                  ▼
┌──────────────┐  ┌──────────────────────────────────┐
│  INFECTION   │  │  WITHIN-HOSPITAL TRANSMISSION     │
│  PROCESS     │  │  Contact rate × colonisation      │
│  Col → BSI   │  │  pressure × hygiene effectiveness │
└──────┬───────┘  └──────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────┐
│            PRESCRIBING PROCESS                       │
│  Antibiotic class selection rates (POLICY LEVER)     │
│  Empiric → targeted transition on culture results    │
└──────────────┬──────────────────────────────────────┘
               │ selective pressure
               ▼
┌─────────────────────────────────────────────────────┐
│          RESISTANCE SELECTION DYNAMICS                │
│  Prescribing pressure shifts R/S ratio               │
│  Fitness cost allows partial reversion               │
│  Horizontal gene transfer component                  │
└─────────────────────────────────────────────────────┘
```

### 2.3 Key modelling choices

- **Two-strain model** initially: susceptible (S) and resistant (R) *E. coli*. Extend to multi-strain / multi-pathogen later.
- **Ward-level granularity** where data permits, trust-level otherwise.
- **Stochastic event rates** learned from data, not assumed from literature. This is the critical methodological contribution.
- **Time resolution:** monthly (matching the prescribing data cadence), with the option to run at finer resolution for within-hospital dynamics.

---

## Phase 3: Learning from Data

### 3.1 Simulation-based inference

Apply the same approach used across all previous stochadex projects:

1. **Smooth and aggregate** the OpenPrescribing and Fingertips data to produce baseline event rates — "what the averaged trust does" in terms of prescribing patterns and resistance trajectories.
2. **Fit deviation coefficients** using simulation-based inference (SBI), matching simulated resistance trajectories to the observed EARS-Net/Fingertips trends conditional on the observed prescribing inputs.
3. **Key parameters to learn:**
   - Within-hospital transmission rate (colonisation acquisition rate per patient-day)
   - Selection coefficient: how much a unit increase in cephalosporin prescribing shifts the R/S ratio
   - Fitness cost of resistance: reversion rate when selective pressure is removed
   - Community importation rate: baseline resistant colonisation at admission

### 3.2 Validation strategy

- **Held-out trusts:** Train on a subset of trusts, validate predictions on others.
- **Temporal holdout:** Train on 2019–2023, predict 2024–2025 resistance trends.
- **Cross-country validation:** Check whether parameters learned from UK data produce plausible trajectories when applied to EARS-Net data from comparable European countries (e.g., Netherlands, Denmark).

---

## Phase 4: Decision Science Layer

### 4.1 Policy actions to evaluate

The decision science layer evaluates candidate prescribing policies — the analogue of rugby substitution timing:

| Policy type | Description |
|-------------|-------------|
| **Antibiotic cycling** | Rotate first-line antibiotic class on a fixed schedule (e.g., quarterly) |
| **Mixing** | Assign different first-line antibiotics to different patients simultaneously |
| **Threshold escalation** | Switch first-line class when local resistance exceeds a threshold (e.g., 10%) |
| **Heterogeneous guidelines** | Different policies for different ward types (ICU vs general medical) |
| **Restriction policies** | Cap broad-spectrum use at a fixed proportion of total prescribing |

### 4.2 Objective function

Simulate multiple trajectories under each policy and evaluate:

- **Primary outcome:** Expected resistant BSI incidence rate at 1, 3, and 5 years
- **Constraint:** Treatment adequacy — empiric therapy must achieve ≥ X% appropriate coverage
- **Secondary outcomes:** Total antibiotic consumption (DDDs), AWaRe category mix, time until resistance threshold is breached

### 4.3 Output

For each trust or trust archetype, produce a policy recommendation analogous to the rugby finding ("front row forwards are best substituted at ~60 minutes"):

> *"For trusts with baseline cephalosporin-resistant E. coli prevalence between 10–15%, switching to a mixing strategy with 40% amoxicillin / 30% nitrofurantoin / 30% trimethoprim as empiric first-line reduces the expected resistant BSI rate by X% over 3 years compared to current guidelines."*

---

## Phase 5: Extensions

Once the core two-strain *E. coli* model is validated:

1. **Multi-pathogen:** Add *K. pneumoniae* (carbapenem resistance — the most concerning EU trend) and *S. aureus* (MRSA)
2. **One Health dimension:** Incorporate agricultural antibiotic use data (EFSA publishes animal-sector AMR data) to model community resistance importation
3. **Network effects:** Model patient transfers between trusts as a transmission pathway, using Hospital Episode Statistics (HES) data
4. **Economic layer:** Add cost data (treatment costs, bed-days, mortality) to produce cost-effectiveness estimates for stewardship interventions
5. **Real-time dashboard:** Connect to live Fingertips/OpenPrescribing data feeds for ongoing trust-level policy recommendations

---

## Concrete First Steps

### Week 1–2: Data acquisition and exploration

- [ ] Pull OpenPrescribing BNF 5.1 (antibacterial) data via API for 5–10 trusts
- [ ] Download matching Fingertips AMR indicators for those trusts
- [ ] Download EARS-Net time series for UK *E. coli* cephalosporin resistance
- [ ] Exploratory analysis: visualise co-movement between prescribing volume and resistance rates

### Week 3–4: Minimal stochadex simulation

- [ ] Implement a two-strain (susceptible/resistant *E. coli*) simulation in the stochadex
- [ ] Define the state transition structure (admission → colonisation → infection → discharge)
- [ ] Implement prescribing-driven selection pressure as an input process
- [ ] Verify the simulation produces qualitatively sensible dynamics with hand-tuned parameters

### Week 5–6: Simulation-based inference

- [ ] Smooth and aggregate the prescribing and resistance data into baseline event rates
- [ ] Set up SBI to learn transmission and selection parameters from the observed data
- [ ] Validate: does the fitted model reproduce held-out trust trajectories?

### Week 7–8: Decision science layer

- [ ] Implement 3–4 candidate prescribing policies as action sets
- [ ] Run policy evaluation: simulate multiple trajectories under each policy
- [ ] Produce initial findings and visualisations
- [ ] Write up as a blog post in the "Engineering Smart Actions in Practice" series

---

## Key Data Sources Summary

| Source | URL | Data type | Access |
|--------|-----|-----------|--------|
| OpenPrescribing | openprescribing.net/api/ | GP & hospital prescribing by trust/practice | Free REST API, no registration |
| UKHSA Fingertips | fingertips.phe.org.uk/profile/amr-local-indicators | AMR indicators by trust/CCG/practice | Free, downloadable |
| ECDC Surveillance Atlas | atlas.ecdc.europa.eu | EARS-Net resistance data, 8 species, EU/EEA | Free, downloadable |
| WHO GLASS Dashboard | who.int/initiatives/glass | Global AMR and antimicrobial consumption | Free, downloadable |
| ESPAUR Reports | gov.uk (search ESPAUR) | Annual UK AMR and prescribing reports with data tables | Free PDF + data tables |
| ResistanceMap | resistancemap.onehealthtrust.org | Global resistance trends, multiple data sources | Free, interactive |

---

## References and Related Work

- `abx_amr_simulator` — Gymnasium-compatible AMR simulation for RL policy testing (bioRxiv, March 2026)
- Pezzani et al. (2024) — Ross-Macdonald model adapted for CRKP hospital transmission (*Scientific Reports*)
- Arepeva et al. (2018) — Systematic review of 38 AMR mathematical models (*Antimicrobial Resistance & Infection Control*)
- Rawson et al. (2020) — System dynamics modelling for antibiotic prescribing policy in hospitals (*J. Operational Research Society*)
- ESPAUR 2024–25 Report — Latest English AMR surveillance data and national action plan progress