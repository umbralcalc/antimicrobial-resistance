#!/usr/bin/env bash
#
# Fetch AMR-related data from the UKHSA Fingertips API.
#
# Indicators downloaded (all for Acute Trust, area_type_id=14):
#   92669 - % E. coli blood specimens with susceptibility tests to 3rd gen cephalosporin (quarterly)
#   92169 - E. coli bacteraemia case counts and rates (annual, by financial year)
#   92193 - E. coli bacteraemia 12-month rolling case counts and rates (monthly)
#   92168 - C. difficile infection case counts and rates (annual)
#   92171 - MRSA bacteraemia case counts and rates (annual)
#   92176 - MSSA bacteraemia case counts and rates (annual)
#   92523 - E. coli bacteraemia hospital-onset case counts and rates (annual)
#
# Prescribing indicators (ICB sub-location, area_type_id=66):
#   92167 - % prescribed antibiotic items from cephalosporin/quinolone/co-amoxiclav (quarterly)
#   91900 - Total prescribed antibiotic items per STAR-PU (quarterly)
#
# Usage: ./dat/fetch_fingertips.sh

set -eo pipefail

BASE_URL="https://fingertips.phe.org.uk/api/all_data/csv/by_indicator_id"
DATA_DIR="$(cd "$(dirname "$0")" && pwd)"

fetch() {
    local name="$1" id="$2" area_type="$3"
    local out="${DATA_DIR}/fingertips_${name}.csv"
    echo "  Fetching indicator ${id} (${name}) for area_type=${area_type}..."
    curl -sf "${BASE_URL}?indicator_ids=${id}&area_type_id=${area_type}" -o "${out}"
    rows=$(wc -l < "${out}")
    echo "    -> $(basename "${out}") (${rows} rows)"
}

echo "Downloading Fingertips AMR data to ${DATA_DIR}/ ..."

# Resistance & infection indicators (Acute Trust, area_type_id=14)
fetch ecoli_cephalosporin_susceptibility 92669 14
fetch ecoli_bacteraemia_annual           92169 14
fetch ecoli_bacteraemia_rolling          92193 14
fetch cdiff_annual                       92168 14
fetch mrsa_annual                        92171 14
fetch mssa_annual                        92176 14
fetch ecoli_hospital_onset_annual        92523 14

# Prescribing indicators (ICB sub-location, area_type_id=66)
fetch broadspectrum_pct                  92167 66
fetch total_antibiotics_starpu           91900 66

echo ""
echo "Done. Downloaded $(ls -1 "${DATA_DIR}"/fingertips_*.csv 2>/dev/null | wc -l) files."
