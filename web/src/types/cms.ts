export interface NavItem {
  label: string;
  path: string;
}

export interface Contact {
  email: string;
  github: string;
  linkedin: string;
}

export interface PublicSettings {
  site_name: string;
  meta_description: string;
  rss_title: string;
  rss_description: string;
  monogram: string;
  nav: NavItem[];
  contact: Contact;
}

export interface Page {
  slug: string;
  title: string;
  meta_description: string;
  body_json: string;
}

export interface WorkItem {
  id: number;
  name: string;
  one_liner: string;
  body: string;
  stack: string[];
  status: string;
  href: string;
  sort_order: number;
}

export interface StudioPiece {
  id: number;
  slug: string;
  title: string;
  year: string;
  medium: string;
  caption: string;
  image_media_id: number | null;
  image_url?: string;
  sort_order: number;
  published: boolean;
}

export interface MediaAsset {
  id: number;
  stored_name: string;
  original_name: string;
  mime: string;
  byte_size: number;
  created_at: string;
  url: string;
}

export interface HomeDomainLink {
  to: string;
  label: string;
}

export interface HomeDomain {
  title: string;
  blurb: string;
  offset: string;
  link: HomeDomainLink | null;
}

export interface HomeBody {
  eyebrow: string;
  headline: string;
  intro: string;
  domains: HomeDomain[];
  now: string;
  /** When true, focus domains collapse to title until expanded. */
  accordion?: boolean;
}

export interface WorkPageBody {
  eyebrow: string;
  headline: string;
  intro: string;
  empty_message: string;
  /** When true, work items collapse detail under the name row. */
  accordion?: boolean;
}

export interface StudioPageBody {
  eyebrow: string;
  headline: string;
  intro: string;
  tools_line: string;
  empty_message: string;
}

export interface NotesPageBody {
  eyebrow: string;
  headline: string;
  intro: string;
  empty_message: string;
}

export interface ResumePageBody {
  eyebrow: string;
  headline: string;
  blurb: string;
  pdf_media_id: number | null;
}

export interface NotFoundPageBody {
  eyebrow: string;
  headline: string;
  body: string;
}

export interface ImportResult {
  settings_upserted: number;
  pages_upserted: number;
  work_created: number;
  studio_created: number;
  sections_created: number;
  entries_created: number;
}
