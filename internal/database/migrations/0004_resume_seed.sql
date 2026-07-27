-- Legacy placeholder resume entries (historical seed; do not add more content migrations).
-- section_id matches 0002 seed order: 1=experience, 2=education, 3=activity.
-- New installs may get these placeholders; replace or delete them via Admin → Resume.
-- Editorial content must not be shipped in migrations — use the admin API or import.

INSERT INTO resume_entries (section_id, org, role, location, period, body_md, body_html, tech, sort_order) VALUES
(
  1,
  'Placeholder Company',
  'Senior Software Engineer',
  'Somewhere (Remote)',
  '20XX–present',
  'Placeholder entry. A real role description will replace this text via the admin panel — what the job was, what shipped, and what it ran on.',
  '<p>Placeholder entry. A real role description will replace this text via the admin panel — what the job was, what shipped, and what it ran on.</p>',
  'Python, Go, SQL',
  1
),
(
  1,
  'Previous Placeholder Co',
  'Software Engineer',
  'Elsewhere (On-Site)',
  '20XX–20XX',
  'Another placeholder entry holding the layout in place. Swap it for the real thing from the admin panel whenever the content is ready.',
  '<p>Another placeholder entry holding the layout in place. Swap it for the real thing from the admin panel whenever the content is ready.</p>',
  'TypeScript, React, Docker',
  2
),
(
  2,
  'Placeholder University',
  'B.Sc. Placeholder Engineering',
  'Somewhere',
  '20XX–20XX',
  '',
  '',
  '',
  1
),
(
  3,
  'Placeholder Activity',
  '',
  '',
  '20XX–present',
  'A short placeholder note about a side interest or community activity.',
  '<p>A short placeholder note about a side interest or community activity.</p>',
  '',
  1
);
