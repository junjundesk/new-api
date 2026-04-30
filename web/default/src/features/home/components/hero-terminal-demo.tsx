import { useState, useEffect, useRef, type ReactNode } from 'react'
import { cn } from '@/lib/utils'

interface ApiDemoConfig {
  id: string
  label: string
  endpoint: string
  requestBodyLines: string[]
  responseKind: 'chat' | 'responses' | 'claude' | 'gemini'
  response: string
  tokens: number
  latency: number
  badgeClass: string
}

const API_DEMOS: ApiDemoConfig[] = [
  {
    id: 'gpt-chat',
    label: 'GPT Chat',
    endpoint: '/v1/chat/completions',
    requestBodyLines: [
      '"model": "your-model",',
      '"messages": [',
      '  { "role": "user", "content": "..." }',
      ']',
    ],
    responseKind: 'chat',
    response: 'Route chat requests through configured upstreams.',
    tokens: 27,
    latency: 142,
    badgeClass:
      'bg-emerald-500/10 text-emerald-600 ring-emerald-500/20 dark:bg-emerald-500/15 dark:text-emerald-400 dark:ring-emerald-500/25',
  },
  {
    id: 'responses',
    label: 'Responses',
    endpoint: '/v1/responses',
    requestBodyLines: ['"model": "your-model",', '"input": "..."'],
    responseKind: 'responses',
    response: 'Run response workflows behind one gateway.',
    tokens: 31,
    latency: 168,
    badgeClass:
      'bg-amber-500/10 text-amber-600 ring-amber-500/20 dark:bg-amber-500/15 dark:text-amber-400 dark:ring-amber-500/25',
  },
  {
    id: 'claude',
    label: 'Claude',
    endpoint: '/v1/messages',
    requestBodyLines: [
      '"model": "your-model",',
      '"max_tokens": 1024,',
      '"messages": [',
      '  { "role": "user", "content": "..." }',
      ']',
    ],
    responseKind: 'claude',
    response: 'Send Claude-style messages through your gateway.',
    tokens: 29,
    latency: 156,
    badgeClass:
      'bg-blue-500/10 text-blue-600 ring-blue-500/20 dark:bg-blue-500/15 dark:text-blue-400 dark:ring-blue-500/25',
  },
  {
    id: 'gemini',
    label: 'Gemini',
    endpoint: '/v1beta/models/{model}:generateContent',
    requestBodyLines: [
      '"contents": [',
      '  { "parts": [{ "text": "..." }] }',
      ']',
    ],
    responseKind: 'gemini',
    response: 'Serve Gemini-compatible generation requests.',
    tokens: 25,
    latency: 93,
    badgeClass:
      'bg-violet-500/10 text-violet-600 ring-violet-500/20 dark:bg-violet-500/15 dark:text-violet-400 dark:ring-violet-500/25',
  },
]

const CYCLE_INTERVAL = 4000

export function HeroTerminalDemo() {
  const [activeIndex, setActiveIndex] = useState(0)
  const [transitioning, setTransitioning] = useState(false)
  const intervalRef = useRef<ReturnType<typeof setInterval>>(undefined)

  useEffect(() => {
    const mq = window.matchMedia('(prefers-reduced-motion: reduce)')
    if (mq.matches) return

    intervalRef.current = setInterval(() => {
      setTransitioning(true)
      setTimeout(() => {
        setActiveIndex((prev) => (prev + 1) % API_DEMOS.length)
        setTransitioning(false)
      }, 300)
    }, CYCLE_INTERVAL)

    return () => clearInterval(intervalRef.current)
  }, [])

  const demo = API_DEMOS[activeIndex]

  return (
    <div className='mx-auto mt-16 w-full max-w-2xl'>
      <div
        className={cn(
          'overflow-hidden rounded-xl border',
          'border-border/60 bg-white shadow-[0_8px_32px_-8px_rgba(0,0,0,0.1),0_0_0_0.5px_rgba(0,0,0,0.04)]',
          'dark:border-border/40 dark:bg-[#0d1117] dark:shadow-[0_8px_32px_-8px_rgba(0,0,0,0.6),0_0_0_0.5px_rgba(255,255,255,0.05)]'
        )}
      >
        {/* Title bar */}
        <div
          className={cn(
            'flex items-center justify-between border-b px-4 py-2.5',
            'border-border/40 bg-gray-50/80',
            'dark:border-white/[0.06] dark:bg-transparent'
          )}
        >
          <div className='flex items-center gap-1.5'>
            <div className='size-2.5 rounded-full bg-[#ff5f57]/80 dark:bg-[#ff5f57]' />
            <div className='size-2.5 rounded-full bg-[#febc2e]/80 dark:bg-[#febc2e]' />
            <div className='size-2.5 rounded-full bg-[#28c840]/80 dark:bg-[#28c840]' />
          </div>
          <div className='flex items-center gap-2'>
            <ModelSelector
              demos={API_DEMOS}
              activeIndex={activeIndex}
              onSelect={(i) => {
                clearInterval(intervalRef.current)
                setTransitioning(true)
                setTimeout(() => {
                  setActiveIndex(i)
                  setTransitioning(false)
                }, 300)
              }}
            />
          </div>
          <div className='flex items-center gap-2'>
            <span className='inline-block size-1.5 rounded-full bg-emerald-500 dark:bg-emerald-400' />
            <span className='text-foreground/30 text-[10px]'>200 OK</span>
          </div>
        </div>

        {/* Terminal body — fixed height */}
        <div className='grid min-h-[280px] grid-rows-[auto_1fr] font-mono text-[12.5px] leading-[1.7]'>
          {/* Request */}
          <div
            className={cn(
              'border-b px-5 py-3.5',
              'border-border/30',
              'dark:border-white/[0.04]'
            )}
          >
            <div className='mb-1.5 flex items-center gap-2'>
              <span className='text-[10px] font-medium tracking-wider text-blue-500/60 uppercase dark:text-blue-400/60'>
                Request
              </span>
            </div>
            <RequestPreview demo={demo} transitioning={transitioning} />
          </div>

          {/* Response */}
          <div className='px-5 py-3.5'>
            <div className='mb-2 flex items-center justify-between'>
              <div className='flex items-center gap-2'>
                <span className='text-[10px] font-medium tracking-wider text-emerald-600/60 uppercase dark:text-emerald-400/60'>
                  Response
                </span>
                <span
                  className={cn(
                    'text-foreground/25 text-[10px] tabular-nums transition-opacity duration-300',
                    transitioning ? 'opacity-0' : 'opacity-100'
                  )}
                >
                  {demo.latency}ms
                </span>
              </div>
              <div
                className={cn(
                  'text-foreground/25 flex items-center gap-3 text-[10px] tabular-nums transition-opacity duration-300',
                  transitioning ? 'opacity-0' : 'opacity-100'
                )}
              >
                <span>{demo.tokens} tokens</span>
                <span>${(demo.tokens * 0.00003).toFixed(5)}</span>
              </div>
            </div>
            <ResponsePreview demo={demo} transitioning={transitioning} />
          </div>
        </div>
      </div>
    </div>
  )
}

function RequestPreview(props: {
  demo: ApiDemoConfig
  transitioning: boolean
}) {
  const { demo, transitioning } = props

  return (
    <div className='space-y-0.5 text-foreground/80'>
      <CodeLine>
        <Command>curl</Command> <Flag>-X POST</Flag>{' '}
        <AnimatedString transitioning={transitioning}>
          &quot;{demo.endpoint}&quot;
        </AnimatedString>{' '}
        <Muted>{'\\'}</Muted>
      </CodeLine>
      <CodeLine indent={2}>
        <Flag>-H</Flag>{' '}
        <StringText>&quot;Authorization: Bearer sk-••••&quot;</StringText>{' '}
        <Muted>{'\\'}</Muted>
      </CodeLine>
      <CodeLine indent={2}>
        <Flag>-d</Flag> <StringText>&apos;{'{'}</StringText>
      </CodeLine>
      {demo.requestBodyLines.map((line) => (
        <CodeLine key={line} indent={4}>
          <AnimatedString transitioning={transitioning}>
            {line}
          </AnimatedString>
        </CodeLine>
      ))}
      <CodeLine indent={2}>
        <StringText>{'}'}&apos;</StringText>
      </CodeLine>
    </div>
  )
}

function ResponsePreview(props: {
  demo: ApiDemoConfig
  transitioning: boolean
}) {
  const { demo, transitioning } = props

  return (
    <div
      className={cn(
        'rounded-lg border px-3.5 py-3',
        'border-border/40 bg-muted/30',
        'dark:border-white/[0.06] dark:bg-white/[0.02]'
      )}
    >
      {demo.responseKind === 'chat' && (
        <>
          <CodeLine>
            <Muted>{'{'}</Muted>
          </CodeLine>
          <CodeLine indent={2}>
            <Key>&quot;choices&quot;</Key>
            <Muted>: [</Muted>
          </CodeLine>
          <CodeLine indent={4}>
            <Muted>{'{'} </Muted>
            <Key>&quot;message&quot;</Key>
            <Muted>: {'{'} </Muted>
            <Key>&quot;content&quot;</Key>
            <Muted>: </Muted>
            <ResponseText demo={demo} transitioning={transitioning} />
            <Muted> {'}'} {'}'}</Muted>
          </CodeLine>
          <CodeLine indent={2}>
            <Muted>],</Muted>
          </CodeLine>
          <UsageLine
            container='usage'
            name='total_tokens'
            value={demo.tokens}
            indent={2}
          />
          <CodeLine>
            <Muted>{'}'}</Muted>
          </CodeLine>
        </>
      )}

      {demo.responseKind === 'responses' && (
        <>
          <CodeLine>
            <Muted>{'{'}</Muted>
          </CodeLine>
          <CodeLine indent={2}>
            <Key>&quot;output&quot;</Key>
            <Muted>: [</Muted>
          </CodeLine>
          <CodeLine indent={4}>
            <Muted>{'{'}</Muted>
          </CodeLine>
          <CodeLine indent={6}>
            <Key>&quot;type&quot;</Key>
            <Muted>: </Muted>
            <StringText>&quot;message&quot;</StringText>
            <Muted>,</Muted>
          </CodeLine>
          <CodeLine indent={6}>
            <Key>&quot;content&quot;</Key>
            <Muted>: [</Muted>
          </CodeLine>
          <CodeLine indent={8}>
            <Muted>{'{'} </Muted>
            <Key>&quot;type&quot;</Key>
            <Muted>: </Muted>
            <StringText>&quot;output_text&quot;</StringText>
            <Muted>, </Muted>
            <Key>&quot;text&quot;</Key>
            <Muted>: </Muted>
            <ResponseText demo={demo} transitioning={transitioning} />
            <Muted> {'}'}</Muted>
          </CodeLine>
          <CodeLine indent={6}>
            <Muted>]</Muted>
          </CodeLine>
          <CodeLine indent={4}>
            <Muted>{'}'}</Muted>
          </CodeLine>
          <CodeLine indent={2}>
            <Muted>],</Muted>
          </CodeLine>
          <UsageLine
            container='usage'
            name='total_tokens'
            value={demo.tokens}
            indent={2}
          />
          <CodeLine>
            <Muted>{'}'}</Muted>
          </CodeLine>
        </>
      )}

      {demo.responseKind === 'claude' && (
        <>
          <CodeLine>
            <Muted>{'{'}</Muted>
          </CodeLine>
          <CodeLine indent={2}>
            <Key>&quot;content&quot;</Key>
            <Muted>: [</Muted>
          </CodeLine>
          <CodeLine indent={4}>
            <Muted>{'{'} </Muted>
            <Key>&quot;type&quot;</Key>
            <Muted>: </Muted>
            <StringText>&quot;text&quot;</StringText>
            <Muted>, </Muted>
            <Key>&quot;text&quot;</Key>
            <Muted>: </Muted>
            <ResponseText demo={demo} transitioning={transitioning} />
            <Muted> {'}'}</Muted>
          </CodeLine>
          <CodeLine indent={2}>
            <Muted>],</Muted>
          </CodeLine>
          <CodeLine indent={2}>
            <Key>&quot;usage&quot;</Key>
            <Muted>: {'{'} </Muted>
            <Key>&quot;input_tokens&quot;</Key>
            <Muted>: </Muted>
            <NumberText>{Math.floor(demo.tokens * 0.4)}</NumberText>
            <Muted>, </Muted>
            <Key>&quot;output_tokens&quot;</Key>
            <Muted>: </Muted>
            <NumberText>{Math.ceil(demo.tokens * 0.6)}</NumberText>
            <Muted> {'}'}</Muted>
          </CodeLine>
          <CodeLine>
            <Muted>{'}'}</Muted>
          </CodeLine>
        </>
      )}

      {demo.responseKind === 'gemini' && (
        <>
          <CodeLine>
            <Muted>{'{'}</Muted>
          </CodeLine>
          <CodeLine indent={2}>
            <Key>&quot;candidates&quot;</Key>
            <Muted>: [</Muted>
          </CodeLine>
          <CodeLine indent={4}>
            <Muted>{'{'}</Muted>
          </CodeLine>
          <CodeLine indent={6}>
            <Key>&quot;content&quot;</Key>
            <Muted>: {'{'}</Muted>
          </CodeLine>
          <CodeLine indent={8}>
            <Key>&quot;parts&quot;</Key>
            <Muted>: [</Muted>
          </CodeLine>
          <CodeLine indent={10}>
            <Muted>{'{'} </Muted>
            <Key>&quot;text&quot;</Key>
            <Muted>: </Muted>
            <ResponseText demo={demo} transitioning={transitioning} />
            <Muted> {'}'}</Muted>
          </CodeLine>
          <CodeLine indent={8}>
            <Muted>]</Muted>
          </CodeLine>
          <CodeLine indent={6}>
            <Muted>{'}'}</Muted>
          </CodeLine>
          <CodeLine indent={4}>
            <Muted>{'}'}</Muted>
          </CodeLine>
          <CodeLine indent={2}>
            <Muted>],</Muted>
          </CodeLine>
          <UsageLine
            container='usageMetadata'
            name='totalTokenCount'
            value={demo.tokens}
            indent={2}
          />
          <CodeLine>
            <Muted>{'}'}</Muted>
          </CodeLine>
        </>
      )}
    </div>
  )
}

function UsageLine(props: {
  container: string
  name: string
  value: number
  indent: number
}) {
  return (
    <CodeLine indent={props.indent}>
      <Key>&quot;{props.container}&quot;</Key>
      <Muted>: {'{'} </Muted>
      <Key>&quot;{props.name}&quot;</Key>
      <Muted>: </Muted>
      <NumberText>{props.value}</NumberText>
      <Muted> {'}'}</Muted>
    </CodeLine>
  )
}

function ResponseText(props: {
  demo: ApiDemoConfig
  transitioning: boolean
}) {
  return (
    <span
      className={cn(
        'text-emerald-600 transition-all duration-300 dark:text-emerald-400',
        props.transitioning ? 'opacity-0' : 'opacity-100'
      )}
    >
      &quot;{props.demo.response}&quot;
    </span>
  )
}

function CodeLine(props: { children: ReactNode; indent?: number }) {
  return (
    <div className='whitespace-pre-wrap break-words'>
      {props.indent ? (
        <span
          aria-hidden
          className='inline-block'
          style={{ width: `${props.indent}ch` }}
        />
      ) : null}
      {props.children}
    </div>
  )
}

function AnimatedString(props: {
  children: ReactNode
  transitioning: boolean
}) {
  return (
    <span
      className={cn(
        'transition-all duration-300',
        props.transitioning
          ? 'text-foreground/20'
          : 'text-amber-700 dark:text-amber-300'
      )}
    >
      {props.children}
    </span>
  )
}

function Command(props: { children: ReactNode }) {
  return (
    <span className='text-emerald-600 dark:text-emerald-400'>
      {props.children}
    </span>
  )
}

function Flag(props: { children: ReactNode }) {
  return (
    <span className='text-blue-600 dark:text-blue-400'>{props.children}</span>
  )
}

function Key(props: { children: ReactNode }) {
  return (
    <span className='text-blue-600 dark:text-blue-400'>{props.children}</span>
  )
}

function StringText(props: { children: ReactNode }) {
  return (
    <span className='text-amber-600 dark:text-amber-400'>{props.children}</span>
  )
}

function NumberText(props: { children: ReactNode }) {
  return (
    <span className='text-violet-600 dark:text-violet-400'>
      {props.children}
    </span>
  )
}

function Muted(props: { children: ReactNode }) {
  return <span className='text-foreground/35'>{props.children}</span>
}

function ModelSelector(props: {
  demos: ApiDemoConfig[]
  activeIndex: number
  onSelect: (index: number) => void
}) {
  return (
    <div className='flex items-center gap-1'>
      {props.demos.map((demo, i) => (
        <button
          key={demo.id}
          onClick={() => props.onSelect(i)}
          className={cn(
            'rounded-md px-1.5 py-0.5 text-[10px] font-medium ring-1 transition-all duration-300 ring-inset',
            i === props.activeIndex
              ? demo.badgeClass
              : 'text-foreground/20 ring-border/30 hover:text-foreground/40 hover:ring-border/50 dark:ring-white/[0.06] dark:hover:ring-white/10'
          )}
        >
          {demo.label}
        </button>
      ))}
    </div>
  )
}
