// dothesplit importer page glue. Same flow as the Splitwise importer (shared
// in import-csv-common.ts); only the preview endpoint differs - it routes to
// /v1/imports/dothesplit so the richer parser handles the extra columns.
import { setupFullImporter } from "./import-csv-common";

setupFullImporter("/api/import-dothesplit-preview");
