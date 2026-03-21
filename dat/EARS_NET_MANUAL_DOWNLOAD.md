# EARS-Net Data (Manual Download)

The ECDC Surveillance Atlas uses an interactive browser interface without a public
API. To download the E. coli cephalosporin resistance time series:

1. Go to https://atlas.ecdc.europa.eu/public/index.aspx
2. Select **Antimicrobial resistance** from the disease dropdown
3. Set filters:
   - Pathogen: *Escherichia coli*
   - Antimicrobial: *Third-generation cephalosporins (combined)*
   - Indicator: *Resistant (%)*
   - Population: *All*
   - Time: 2019–2025
   - Region: United Kingdom (or all countries for cross-validation)
4. Click the **Table** tab, then **Export** → **CSV file**
5. Save as `dat/ears_net_ecoli_cephalosporin.csv`

This data is used for cross-country validation (Phase 3.2 of the project plan),
not for primary model fitting. The Fingertips data provides higher-resolution
UK trust-level data for that purpose.
