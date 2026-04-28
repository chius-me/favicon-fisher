import { getFaviconData } from '@/lib/favicon';

type SearchParams = Promise<{
  url?: string;
}>;

export default async function Home({
  searchParams,
}: {
  searchParams?: SearchParams;
}) {
  const params = (await searchParams) ?? {};
  const inputUrl = params.url?.trim() ?? '';

  let result: Awaited<ReturnType<typeof getFaviconData>> | null = null;
  let error: string | null = null;

  if (inputUrl) {
    try {
      result = await getFaviconData(inputUrl);
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'Something went wrong.';
    }
  }

  return (
    <main className="page-shell">
      <section className="hero-card">
        <p className="eyebrow">favicon-fisher</p>
        <h1>输入网站地址，抓它正在用的图标</h1>
        <p className="subtitle">
          会优先解析页面里声明的 icon、apple-touch-icon、manifest 图标，最后再回退到 /favicon.ico。
        </p>

        <form className="search-form" action="/" method="get">
          <input
            aria-label="Website URL"
            className="url-input"
            defaultValue={inputUrl}
            name="url"
            placeholder="example.com"
            type="text"
          />
          <button className="submit-button" type="submit">
            抓取图标
          </button>
        </form>

        {!inputUrl ? (
          <div className="empty-state">
            <span>试试：</span>
            <code>github.com</code>
            <code>vercel.com</code>
            <code>apple.com</code>
          </div>
        ) : null}
      </section>

      {error ? <section className="result-card error-card">{error}</section> : null}

      {result ? (
        <section className="result-card">
          <div className="result-header">
            <div>
              <p className="result-label">Best match</p>
              <h2>{result.title ?? result.siteUrl}</h2>
              <a href={result.siteUrl} rel="noreferrer" target="_blank">
                {result.siteUrl}
              </a>
            </div>

            {result.bestIcon ? (
              <div className="icon-preview">
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img alt="Best favicon" src={result.bestIcon.url} />
              </div>
            ) : null}
          </div>

          {result.bestIcon ? (
            <div className="best-icon-meta">
              <span>{result.bestIcon.rel}</span>
              <span>{result.bestIcon.sizes ?? 'unknown size'}</span>
              <a href={result.bestIcon.url} rel="noreferrer" target="_blank">
                打开原图
              </a>
            </div>
          ) : null}

          <div className="candidate-grid">
            {result.candidates.map((candidate) => (
              <article className="candidate-card" key={`${candidate.url}-${candidate.rel}-${candidate.sizes ?? 'na'}`}>
                <div className="candidate-image">
                  {/* eslint-disable-next-line @next/next/no-img-element */}
                  <img alt={candidate.rel} src={candidate.url} />
                </div>
                <div className="candidate-body">
                  <strong>{candidate.rel}</strong>
                  <span>{candidate.sizes ?? 'unknown size'}</span>
                  <span>{candidate.source}</span>
                  <a href={candidate.url} rel="noreferrer" target="_blank">
                    {candidate.url}
                  </a>
                </div>
              </article>
            ))}
          </div>
        </section>
      ) : null}
    </main>
  );
}
