import { useState, useEffect } from 'react'

const API = 'http://127.0.0.1:8090'

export default function App() {
  const [environments, setEnvironments] = useState([])
  const [creating, setCreating] = useState(false)
  const [form, setForm] = useState({ type: 'ec2', ttlMinutes: '', vars: {} })
  const [error, setError] = useState(null)

  // Poll for environments every 5 seconds
  useEffect(() => {
    fetchEnvironments()
    const interval = setInterval(fetchEnvironments, 5000)
    return () => clearInterval(interval)
  }, [])

  async function fetchEnvironments() {
    try {
      const res = await fetch(`${API}/environments`)
      const data = await res.json()
      // Fetch status for each environment
      const withStatus = await Promise.all(
        data.map(async (env) => {
          try {
            const statusRes = await fetch(`${API}/environments/${env.workflowId}`)
            const status = await statusRes.json()
            return { ...env, ...status }
          } catch {
            return env
          }
        })
      )
      setEnvironments(withStatus)
    } catch (e) {
      console.error('Failed to fetch environments', e)
    }
  }

  async function createEnvironment(e) {
    e.preventDefault()
    setCreating(true)
    setError(null)
    try {
      const body = { type: form.type }
      if (form.ttlMinutes) body.ttlMinutes = parseInt(form.ttlMinutes)
      const vars = Object.fromEntries(Object.entries(form.vars).filter(([, v]) => v !== ''))
      if (Object.keys(vars).length > 0) body.vars = vars

      const res = await fetch(`${API}/environments`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })
      if (!res.ok) throw new Error(await res.text())
      setForm({ type: 'ec2', ttlMinutes: '', vars: {} })
      fetchEnvironments()
    } catch (e) {
      setError(e.message)
    } finally {
      setCreating(false)
    }
  }

  async function destroyEnvironment(id) {
    await fetch(`${API}/environments/${id}`, { method: 'DELETE' })
    fetchEnvironments()
  }

  async function extendEnvironment(id, minutes) {
    await fetch(`${API}/environments/${id}/extend`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ minutes }),
    })
    fetchEnvironments()
  }

  return (
    <div className="min-h-screen bg-gray-950 text-gray-100 p-8">
      <div className="max-w-4xl mx-auto space-y-8">

        {/* Header */}
        <div>
          <h1 className="text-3xl font-bold text-white">Temporal Environment Manager</h1>
          <p className="text-gray-400 mt-1">Provision and manage temporary or permanent AWS environments</p>
        </div>

        {/* Create Form */}
        <div className="bg-gray-900 rounded-xl p-6 border border-gray-800">
          <h2 className="text-lg font-semibold mb-4">Create Environment</h2>
          <form onSubmit={createEnvironment} className="space-y-6">
            <div className="grid grid-cols-2 divide-x divide-gray-800">

              {/* Left column: Type */}
              <div className="flex flex-col gap-3 pr-6">
                <span className="text-xs font-semibold text-gray-400 uppercase tracking-widest">Type</span>
                {[{ value: 'ec2', label: 'EC2' }, { value: 's3lambda', label: 'S3 + Lambda' }].map(({ value, label }) => (
                  <label
                    key={value}
                    className={`flex items-center gap-3 px-4 py-3 rounded-lg border cursor-pointer transition-colors ${
                      form.type === value
                        ? 'border-blue-500 bg-blue-950 text-white'
                        : 'border-gray-700 bg-gray-800 text-gray-400 hover:border-gray-500'
                    }`}
                  >
                    <input
                      type="radio"
                      name="type"
                      value={value}
                      checked={form.type === value}
                      onChange={() => setForm({ ...form, type: value, vars: {} })}
                      className="accent-blue-500"
                    />
                    {label}
                  </label>
                ))}
              </div>

              {/* Right column: Inputs */}
              <div className="flex flex-col gap-4 pl-6">
                <span className="text-xs font-semibold text-gray-400 uppercase tracking-widest">Inputs</span>

                <div className="flex flex-col gap-1">
                  <label className="text-sm text-gray-400">TTL (minutes)</label>
                  <input
                    type="number"
                    placeholder="No TTL — runs until destroyed"
                    value={form.ttlMinutes}
                    onChange={e => setForm({ ...form, ttlMinutes: e.target.value })}
                    className="bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white placeholder-gray-500 focus:outline-none focus:border-blue-500"
                  />
                </div>

                <VarFields type={form.type} vars={form.vars} onChange={vars => setForm({ ...form, vars })} />
              </div>

            </div>

            <button
              type="submit"
              disabled={creating}
              className="bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white font-medium px-5 py-2 rounded-lg transition-colors"
            >
              {creating ? 'Creating...' : 'Create'}
            </button>

          </form>
          {error && <p className="text-red-400 text-sm mt-3">{error}</p>}
        </div>

        {/* Environments List */}
        <div className="bg-gray-900 rounded-xl border border-gray-800">
          <div className="p-6 border-b border-gray-800 flex items-center justify-between">
            <h2 className="text-lg font-semibold">Running Environments</h2>
            <span className="text-sm text-gray-400">{environments.length} active</span>
          </div>

          {environments.length === 0 ? (
            <p className="text-gray-500 text-sm p-6">No running environments.</p>
          ) : (
            <div className="divide-y divide-gray-800">
              {environments.map(env => (
                <EnvironmentRow
                  key={env.workflowId}
                  env={env}
                  onDestroy={() => destroyEnvironment(env.workflowId)}
                  onExtend={(mins) => extendEnvironment(env.workflowId, mins)}
                />
              ))}
            </div>
          )}
        </div>

      </div>
    </div>
  )
}

function EnvironmentRow({ env, onDestroy, onExtend }) {
  const [extendMins, setExtendMins] = useState(10)

  const outputs = [
    { label: 'Started At', value: new Date(env.startTime).toLocaleString() },
    env.instanceId         && { label: 'Instance ID',       value: env.instanceId },
    env.vpcId              && { label: 'VPC ID',            value: env.vpcId },
    env.bucketName         && { label: 'Bucket Name',       value: env.bucketName },
    env.lambdaFunctionName && { label: 'Lambda Function',   value: env.lambdaFunctionName },
    env.lambdaArn          && { label: 'Lambda ARN',        value: env.lambdaArn },
  ].filter(Boolean)

  return (
    <div className="p-6 space-y-4">

      {/* Header row: ID + badges + actions */}
      <div className="flex flex-wrap gap-4 items-center justify-between">
        <div className="flex items-center gap-3">
          <span className="font-mono text-sm text-white">{env.workflowId}</span>
          <TypeBadge type={env.type} />
          <StepBadge step={env.step} />
        </div>

        {/* Actions */}
        <div className="flex items-center gap-2">
          <div className="flex items-center gap-1">
            <span className="text-xs text-gray-400">Extend by</span>
            <input
              type="number"
              value={extendMins}
              onChange={e => setExtendMins(parseInt(e.target.value))}
              className="bg-gray-800 border border-gray-700 rounded-lg px-2 py-1 text-white text-sm w-16 focus:outline-none focus:border-blue-500"
            />
            <span className="text-xs text-gray-400">min</span>
          </div>
          <button
            onClick={() => onExtend(extendMins)}
            className="bg-gray-700 hover:bg-gray-600 text-white text-sm px-3 py-1 rounded-lg transition-colors"
          >
            Extend
          </button>
          <button
            onClick={onDestroy}
            disabled={env.step !== 'sleeping'}
            className="bg-red-700 hover:bg-red-600 disabled:opacity-40 disabled:cursor-not-allowed text-white text-sm px-3 py-1 rounded-lg transition-colors"
          >
            Destroy
          </button>
        </div>
      </div>

      {/* Outputs grid */}
      <div className="grid grid-cols-2 sm:grid-cols-3 gap-x-6 gap-y-4">
        {outputs.map(({ label, value }) => (
          <div key={label}>
            <p className="text-xs text-gray-500 uppercase tracking-wide">{label}</p>
            <p className="text-xs text-gray-200 font-mono break-all">{value}</p>
          </div>
        ))}
      </div>

    </div>
  )
}

const EC2_VARS = [
  { key: 'region', label: 'Region', placeholder: 'us-east-1' },
  { key: 'instanceType', label: 'Instance Type', placeholder: 't2.micro' },
]

const S3LAMBDA_VARS = [
  { key: 'region', label: 'Region', placeholder: 'us-east-1' },
  { key: 'bucketName', label: 'Bucket Name', placeholder: 'temporal-poc-s3lambda-bucket' },
  { key: 'lambdaFunctionName', label: 'Lambda Function Name', placeholder: 'temporal-poc-s3-handler' },
]

function VarFields({ type, vars, onChange }) {
  const fields = type === 'ec2' ? EC2_VARS : S3LAMBDA_VARS
  return (
    <div className="flex flex-col gap-3">
      {fields.map(({ key, label, placeholder }) => (
        <div key={key} className="flex flex-col gap-1">
          <label className="text-sm text-gray-400">{label}</label>
          <input
            type="text"
            placeholder={placeholder}
            value={vars[key] ?? ''}
            onChange={e => onChange({ ...vars, [key]: e.target.value })}
            className="bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white placeholder-gray-500 focus:outline-none focus:border-blue-500 w-52"
          />
        </div>
      ))}
    </div>
  )
}

function TypeBadge({ type }) {
  const styles = {
    ec2: 'bg-orange-900 text-orange-300',
    s3lambda: 'bg-purple-900 text-purple-300',
  }
  return (
    <span className={`text-xs font-medium px-2 py-0.5 rounded-full ${styles[type] ?? 'bg-gray-700 text-gray-300'}`}>
      {type}
    </span>
  )
}

function StepBadge({ step }) {
  const styles = {
    initializing: 'bg-gray-700 text-gray-300',
    applying: 'bg-yellow-900 text-yellow-300',
    'waiting-for-instance': 'bg-yellow-900 text-yellow-300',
    'running-setup': 'bg-yellow-900 text-yellow-300',
    sleeping: 'bg-green-900 text-green-300',
    destroying: 'bg-red-900 text-red-300',
    completed: 'bg-gray-700 text-gray-400',
  }
  return (
    <span className={`text-xs font-medium px-2 py-0.5 rounded-full ${styles[step] ?? 'bg-gray-700 text-gray-300'}`}>
      {step ?? 'unknown'}
    </span>
  )
}
