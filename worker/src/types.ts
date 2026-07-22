export interface Env {
  ASSETS: Fetcher;
  FVF_SIGNING_SECRET?: string;
  FVF_ALLOW_PRIVATE?: string;
}

export interface IconCandidate {
  url: string;
  rel: string;
  sizes: string;
  type: string;
  priority: number;
}

export interface ManifestIcon {
  src?: string;
  sizes?: string;
  type?: string;
}

export interface Manifest {
  icons?: ManifestIcon[];
}

export interface PreviewPayload {
  input_url: string;
  page_url: string;
  recommended_icon_url: string | null;
  icons: {
    icon_url: string;
    token: string;
    source_rel: string;
    sizes: string | null;
    content_type: string;
    allowed_types: string[];
  }[];
}
