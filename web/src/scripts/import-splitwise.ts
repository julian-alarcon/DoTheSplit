// Splitwise importer page glue. The whole two-phase flow lives in
// import-csv-common.ts; this page only differs by its preview endpoint.
import { setupFullImporter } from "./import-csv-common";

setupFullImporter("/api/import-splitwise-preview");
