-- A project can deploy several apps, each with its own CI/CD pipeline. The
-- links live as a JSON array of {"label", "url"} objects; cicd_url stays as the
-- first link's URL so clients that predate cicd_links keep working.
ALTER TABLE projects ADD COLUMN cicd_links TEXT;

-- Existing single URLs become one unlabeled link.
UPDATE projects
SET cicd_links = json_array(json_object('label', '', 'url', cicd_url))
WHERE cicd_url IS NOT NULL AND cicd_url <> '';
