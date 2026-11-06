import { ShieldCheck } from 'lucide-react'
import { useId, useState, type FormEvent } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ScanAnswer } from './ScanAnswer'

const repository = 'https://github.com/lucifer-135/freshness-guard'

interface HomePageProps {
  notice: string | null
  onConnect: (token: string) => Promise<string | null>
}

// HomePage is what a visitor sees before signing in: what the product does,
// one checked answer, how to switch it on, what was measured, and the form
// that opens the chat. It makes no API call until the form is sent.
export function HomePage({ notice, onConnect }: HomePageProps) {
  return (
    <div className="min-h-dvh bg-bg text-text">
      <header className="mx-auto flex max-w-[1200px] flex-wrap items-center gap-x-6 gap-y-2 px-6 py-4 lg:px-12">
        <p className="flex items-center gap-2 font-semibold">
          <ShieldCheck aria-hidden="true" className="size-5" />
          Freshness Guard
        </p>
        <p className="hidden text-sm text-text-subtle md:block">Plugin for the Bifrost LLM gateway</p>
        <nav aria-label="Page" className="ml-auto">
          <ul className="flex gap-5 text-sm text-text-muted">
            <li>
              <a href="#how" className="hover:text-text">
                How it works
              </a>
            </li>
            <li>
              <a href="#measured" className="hover:text-text">
                Benchmark
              </a>
            </li>
            <li>
              <a href={repository} className="hover:text-text">
                Source
              </a>
            </li>
          </ul>
        </nav>
      </header>

      <main className="mx-auto max-w-[1200px] px-6 lg:px-12">
        <section aria-labelledby="home-title" className="pt-10 lg:pt-16">
          <p className="mb-8 max-w-[60ch] text-sm text-text-muted">
            An answer from a language model, checked as it leaves the gateway. The example is built from claims in the curated benchmark.
          </p>
          <ScanAnswer />
          <div className="mt-14 grid gap-8 border-t border-border pt-8 lg:grid-cols-[1fr_minmax(0,26rem)] lg:gap-16">
            <h1 id="home-title" className="max-w-[46ch] text-2xl leading-snug font-semibold text-balance">
              Freshness Guard finds the facts in an LLM answer that may have changed, checks them against live sources, and marks or
              corrects the outdated ones before your users read them.
            </h1>
            <SignInForm notice={notice} onConnect={onConnect} />
          </div>
        </section>

        <section id="how" aria-labelledby="how-title" className="scroll-mt-8 pt-24">
          <h2 id="how-title" className="text-xl font-semibold">
            How it works
          </h2>
          <ol className="mt-6 grid gap-8 md:grid-cols-3 md:gap-0 md:divide-x md:divide-border">
            <li className="md:pr-8">
              <h3 className="font-medium">Find the claims</h3>
              <p className="mt-1 text-text-muted">
                A model reads each answer, lists its factual claims, and estimates how likely each one is to have changed since the answering
                model's knowledge was collected.
              </p>
            </li>
            <li className="md:px-8">
              <h3 className="font-medium">Check the likely changes</h3>
              <p className="mt-1 text-text-muted">
                Those claims are searched on the live web with Tavily, and a judge model compares them with dated sources. Every verdict keeps
                its evidence.
              </p>
            </li>
            <li className="md:pl-8">
              <h3 className="font-medium">Mark, correct or block</h3>
              <p className="mt-1 text-text-muted">
                In the background the answer returns at once and is marked when the checks finish. Before the reply, outdated values are
                corrected or the answer is blocked.
              </p>
            </li>
          </ol>
          <div className="mt-10 grid gap-6 lg:grid-cols-[1fr_minmax(0,26rem)] lg:gap-16">
            <p className="max-w-[60ch] text-text-muted">
              It runs inside Bifrost, so every application behind the gateway is checked without code changes. Two request headers choose when
              to check and what to do with an outdated claim.
            </p>
            <pre className="overflow-x-auto rounded-md border border-border bg-surface-1 p-4 font-mono text-sm leading-relaxed">
              <code className="block">x-bf-freshness: async | sync | off</code>
              <code className="block">x-bf-freshness-policy: annotate | heal | block</code>
            </pre>
          </div>
        </section>

        <section id="measured" aria-labelledby="measured-title" className="scroll-mt-8 pt-24">
          <h2 id="measured-title" className="text-xl font-semibold">
            What was measured
          </h2>
          <p className="mt-2 max-w-[70ch] text-text-muted">
            A curated set of 51 claims about prices, versions, leadership and laws, 27 of them outdated, run on 5 November 2026 with free-tier
            models. The labels were proposed with AI help and are not yet independently reviewed.
          </p>
          <table className="mt-6 w-full max-w-2xl text-left">
            <caption className="sr-only">Benchmark results</caption>
            <tbody className="divide-y divide-border border-y border-border">
              {[
                ['Precision for outdated claims', '0.96'],
                ['Recall for outdated claims', '0.96'],
                ['Time to a verdict, median and 95th percentile', '4.2 s · 6.9 s'],
                ['Tavily credits for the run', '53'],
                ['Added reply latency in background mode, 99th percentile, test stack', 'within noise'],
              ].map(([label, value]) => (
                <tr key={label}>
                  <th scope="row" className="py-3 pr-6 font-normal text-text-muted">
                    {label}
                  </th>
                  <td className="py-3 text-right font-mono">{value}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>
      </main>

      <footer className="mx-auto mt-24 flex max-w-[1200px] flex-wrap gap-x-6 gap-y-2 border-t border-border px-6 py-6 text-sm text-text-subtle lg:px-12">
        <p>Built for the Global AI Hackathon 2026 with Bifrost and Tavily.</p>
        <p>Apache License 2.0</p>
        <a href={repository} className="ml-auto text-link underline underline-offset-4 hover:text-link-strong">
          Source code
        </a>
      </footer>
    </div>
  )
}

function SignInForm({ notice, onConnect }: HomePageProps) {
  const [token, setToken] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const errorId = useId()
  const helpId = useId()

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(await onConnect(token.trim()))
    setBusy(false)
  }

  return (
    <form onSubmit={submit} aria-labelledby="try-title" className="space-y-3">
      <h2 id="try-title" className="font-semibold">
        Try it
      </h2>
      {notice && <p className="text-sm text-text">{notice}</p>}
      <div className="space-y-1.5">
        <Label htmlFor="api-token" className="text-text-muted">
          API token
        </Label>
        <div className="flex gap-2">
          <Input
            id="api-token"
            type="password"
            autoComplete="current-password"
            value={token}
            onChange={(e) => setToken(e.target.value)}
            aria-invalid={error !== null}
            aria-describedby={error ? `${errorId} ${helpId}` : helpId}
            required
            className="h-10"
          />
          <Button type="submit" className="h-10 shrink-0" disabled={busy || token.trim() === ''}>
            Open the chat
          </Button>
        </div>
      </div>
      {error && (
        <p id={errorId} role="alert" className="text-sm text-status-outdated">
          {error}
        </p>
      )}
      <p id={helpId} className="text-xs text-text-subtle">
        Judges: use the token from the submission. Self-hosted: the value of FG_API_TOKEN in deploy/.env. The token stays in this browser
        until you sign out.
      </p>
    </form>
  )
}
