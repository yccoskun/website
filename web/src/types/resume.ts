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
  accordion: boolean;
  entries: ResumeEntry[];
}

/** Resume page header chrome. */
export interface ResumeHeader {
  eyebrow: string;
  headline: string;
  blurb: string;
  pdf_media_id: number | null;
  pdf_url?: string;
}

/** Grouped public resume payload. */
export interface Resume {
  header: ResumeHeader;
  sections: ResumeSection[];
}
