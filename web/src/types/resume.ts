export type ResumeSectionKind = "experience" | "education" | "activity";

/** Single resume entry under a section. */
export interface ResumeEntry {
  id: number;
  section_id: number;
  org: string;
  role: string;
  location: string;
  period: string;
  body_md: string;
  body_html: string;
  tech: string;
  sort_order: number;
}

/** Resume section with nested entries. */
export interface ResumeSection {
  id: number;
  kind: ResumeSectionKind;
  title: string;
  sort_order: number;
  entries: ResumeEntry[];
}

/** Grouped public resume payload. */
export interface Resume {
  sections: ResumeSection[];
}
