// Minimal ISO/IEC 18004 QR Code (Byte mode, ECC Level L, Versions 1-10) SVG data URL generator.
const GF_EXP = new Uint8Array(512)
const GF_LOG = new Uint8Array(256)
{
  let x = 1
  for (let i = 0; i < 255; i += 1) {
    GF_EXP[i] = x
    GF_LOG[x] = i
    x <<= 1
    if (x & 0x100) x ^= 0x11d
  }
  for (let i = 255; i < 512; i += 1) {
    GF_EXP[i] = GF_EXP[i - 255]
  }
}

function gfMul(a: number, b: number) {
  if (a === 0 || b === 0) return 0
  return GF_EXP[GF_LOG[a] + GF_LOG[b]]
}

function rsGenerator(degree: number): Uint8Array {
  let poly = new Uint8Array([1])
  for (let i = 0; i < degree; i += 1) {
    const next = new Uint8Array(poly.length + 1)
    const factor = GF_EXP[i]
    for (let j = 0; j < poly.length; j += 1) {
      next[j] ^= poly[j]
      next[j + 1] ^= gfMul(poly[j], factor)
    }
    poly = next
  }
  return poly
}

function rsEncode(data: Uint8Array, ecCount: number): Uint8Array {
  const gen = rsGenerator(ecCount)
  const buf = new Uint8Array(data.length + ecCount)
  buf.set(data)
  for (let i = 0; i < data.length; i += 1) {
    const factor = buf[i]
    if (factor === 0) continue
    for (let j = 0; j < gen.length; j += 1) {
      buf[i + j] ^= gfMul(gen[j], factor)
    }
  }
  return buf.slice(data.length)
}

// [totalCodewords, ecPerBlock, blocksGroup1, dataPerBlock1, blocksGroup2, dataPerBlock2] for ECC Level L, Versions 1..10
const VERSION_SPECS: ReadonlyArray<readonly [number, number, number, number, number, number]> = [
  [0, 0, 0, 0, 0, 0],
  [26, 7, 1, 19, 0, 0],
  [44, 10, 1, 34, 0, 0],
  [70, 15, 1, 55, 0, 0],
  [100, 20, 1, 80, 0, 0],
  [134, 26, 1, 108, 0, 0],
  [172, 18, 2, 68, 0, 0],
  [196, 20, 2, 78, 0, 0],
  [242, 24, 2, 97, 0, 0],
  [292, 30, 2, 116, 0, 0],
  [346, 18, 2, 68, 2, 69],
]

const ALIGN_POSITIONS: ReadonlyArray<readonly number[]> = [
  [],
  [],
  [6, 18],
  [6, 22],
  [6, 26],
  [6, 30],
  [6, 34],
  [6, 22, 38],
  [6, 24, 42],
  [6, 26, 46],
  [6, 28, 50],
]

function chooseVersion(byteLen: number): number {
  for (let v = 1; v <= 10; v += 1) {
    const [, , b1, d1, b2, d2] = VERSION_SPECS[v]
    const totalDataBytes = b1 * d1 + b2 * d2
    const headerBits = 4 + (v >= 10 ? 16 : 8)
    if (headerBits + byteLen * 8 <= totalDataBytes * 8) {
      return v
    }
  }
  return 0
}

function buildCodewords(bytes: Uint8Array, version: number): Uint8Array {
  const [, ecPerBlock, b1, d1, b2, d2] = VERSION_SPECS[version]
  const totalDataBytes = b1 * d1 + b2 * d2
  const bits: number[] = []
  const pushBits = (val: number, len: number) => {
    for (let i = len - 1; i >= 0; i -= 1) bits.push((val >>> i) & 1)
  }
  pushBits(0b0100, 4)
  pushBits(bytes.length, version >= 10 ? 16 : 8)
  for (const byte of bytes) pushBits(byte, 8)
  const capBits = totalDataBytes * 8
  for (let i = 0; i < 4 && bits.length < capBits; i += 1) bits.push(0)
  while (bits.length % 8 !== 0) bits.push(0)
  const data = new Uint8Array(totalDataBytes)
  for (let i = 0; i < bits.length; i += 8) {
    let b = 0
    for (let j = 0; j < 8; j += 1) b = (b << 1) | (bits[i + j] ?? 0)
    data[i >> 3] = b
  }
  let padIdx = bits.length >> 3
  let toggle = true
  while (padIdx < totalDataBytes) {
    data[padIdx++] = toggle ? 0xec : 0x11
    toggle = !toggle
  }

  const dataBlocks: Uint8Array[] = []
  const ecBlocks: Uint8Array[] = []
  let offset = 0
  for (let i = 0; i < b1; i += 1) {
    const block = data.slice(offset, offset + d1)
    offset += d1
    dataBlocks.push(block)
    ecBlocks.push(rsEncode(block, ecPerBlock))
  }
  for (let i = 0; i < b2; i += 1) {
    const block = data.slice(offset, offset + d2)
    offset += d2
    dataBlocks.push(block)
    ecBlocks.push(rsEncode(block, ecPerBlock))
  }

  const maxData = Math.max(d1, d2)
  const result: number[] = []
  for (let i = 0; i < maxData; i += 1) {
    for (const block of dataBlocks) {
      if (i < block.length) result.push(block[i])
    }
  }
  for (let i = 0; i < ecPerBlock; i += 1) {
    for (const ec of ecBlocks) result.push(ec[i])
  }
  return new Uint8Array(result)
}

function createMatrix(version: number) {
  const size = version * 4 + 17
  const modules = Array.from({ length: size }, () => new Uint8Array(size))
  const isFunc = Array.from({ length: size }, () => new Uint8Array(size))

  const setFn = (r: number, c: number, dark: boolean) => {
    if (r < 0 || r >= size || c < 0 || c >= size) return
    modules[r][c] = dark ? 1 : 0
    isFunc[r][c] = 1
  }

  const placeFinder = (top: number, left: number) => {
    for (let dr = -1; dr <= 7; dr += 1) {
      for (let dc = -1; dc <= 7; dc += 1) {
        const dark =
          dr >= 0 && dr <= 6 && dc >= 0 && dc <= 6 &&
          (dr === 0 || dr === 6 || dc === 0 || dc === 6 || (dr >= 2 && dr <= 4 && dc >= 2 && dc <= 4))
        setFn(top + dr, left + dc, dark)
      }
    }
  }

  placeFinder(0, 0)
  placeFinder(0, size - 7)
  placeFinder(size - 7, 0)

  for (let i = 8; i < size - 8; i += 1) {
    setFn(6, i, i % 2 === 0)
    setFn(i, 6, i % 2 === 0)
  }

  const align = ALIGN_POSITIONS[version]
  for (const ar of align) {
    for (const ac of align) {
      if (isFunc[ar][ac]) continue
      for (let dr = -2; dr <= 2; dr += 1) {
        for (let dc = -2; dc <= 2; dc += 1) {
          setFn(ar + dr, ac + dc, Math.max(Math.abs(dr), Math.abs(dc)) !== 1)
        }
      }
    }
  }

  setFn(4 * version + 9, 8, true)

  // Reserve format info cells
  for (let i = 0; i < 9; i += 1) {
    if (i !== 6) {
      isFunc[8][i] = 1
      isFunc[i][8] = 1
    }
  }
  for (let i = 0; i < 8; i += 1) {
    isFunc[8][size - 1 - i] = 1
    isFunc[size - 1 - i][8] = 1
  }

  // Version info for >= 7
  if (version >= 7) {
    let rem = version
    for (let i = 0; i < 12; i += 1) rem = (rem << 1) ^ ((rem >>> 11) * 0x1f25)
    const bits = (version << 12) | rem
    for (let i = 0; i < 18; i += 1) {
      const dark = ((bits >>> i) & 1) !== 0
      const a = size - 11 + (i % 3)
      const b = Math.floor(i / 3)
      setFn(a, b, dark)
      setFn(b, a, dark)
    }
  }

  return { size, modules, isFunc }
}

function drawFormatBits(modules: Uint8Array[], size: number, mask: number) {
  // ECC Level L = 01
  const data = (0b01 << 3) | mask
  let rem = data
  for (let i = 0; i < 10; i += 1) rem = (rem << 1) ^ ((rem >>> 9) * 0x537)
  const bits = ((data << 10) | rem) ^ 0x5412
  modules[size - 8][8] = 1
  for (let i = 0; i < 15; i += 1) {
    const bit = ((bits >>> i) & 1) !== 0 ? 1 : 0
    if (i < 6) {
      modules[i][8] = bit
    } else if (i < 8) {
      modules[i + 1][8] = bit
    } else if (i === 8) {
      modules[8][7] = bit
    } else {
      modules[8][14 - i] = bit
    }
    if (i < 8) {
      modules[8][size - 1 - i] = bit
    } else {
      modules[size - 15 + i][8] = bit
    }
  }
}

function maskBit(mask: number, r: number, c: number): boolean {
  switch (mask) {
    case 0: return (r + c) % 2 === 0
    case 1: return r % 2 === 0
    case 2: return c % 3 === 0
    case 3: return (r + c) % 3 === 0
    case 4: return (Math.floor(r / 2) + Math.floor(c / 3)) % 2 === 0
    case 5: return ((r * c) % 2) + ((r * c) % 3) === 0
    case 6: return (((r * c) % 2) + ((r * c) % 3)) % 2 === 0
    default: return (((r + c) % 2) + ((r * c) % 3)) % 2 === 0
  }
}

export function encodeQRCodeMatrix(text: string): boolean[][] | null {
  const value = text.trim()
  if (!value) return null
  const bytes = new TextEncoder().encode(value)
  const version = chooseVersion(bytes.length)
  if (!version) return null
  const codewords = buildCodewords(bytes, version)
  const { size, modules, isFunc } = createMatrix(version)

  let bitIdx = 0
  const totalBits = codewords.length * 8
  let upward = true
  for (let right = size - 1; right >= 1; right -= 2) {
    if (right === 6) right = 5
    for (let step = 0; step < size; step += 1) {
      const r = upward ? size - 1 - step : step
      for (let dx = 0; dx < 2; dx += 1) {
        const c = right - dx
        if (isFunc[r][c]) continue
        let dark = false
        if (bitIdx < totalBits) {
          dark = ((codewords[bitIdx >> 3] >>> (7 - (bitIdx & 7))) & 1) !== 0
          bitIdx += 1
        }
        modules[r][c] = dark ? 1 : 0
      }
    }
    upward = !upward
  }

  let bestMask = 0
  let bestPenalty = Infinity
  for (let mask = 0; mask < 8; mask += 1) {
    const candidate = modules.map((row) => row.slice())
    for (let r = 0; r < size; r += 1) {
      for (let c = 0; c < size; c += 1) {
        if (!isFunc[r][c] && maskBit(mask, r, c)) {
          candidate[r][c] ^= 1
        }
      }
    }
    drawFormatBits(candidate, size, mask)
    const penalty = evaluatePenalty(candidate, size)
    if (penalty < bestPenalty) {
      bestPenalty = penalty
      bestMask = mask
    }
  }

  const finalModules = modules.map((row) => row.slice())
  for (let r = 0; r < size; r += 1) {
    for (let c = 0; c < size; c += 1) {
      if (!isFunc[r][c] && maskBit(bestMask, r, c)) {
        finalModules[r][c] ^= 1
      }
    }
  }
  drawFormatBits(finalModules, size, bestMask)
  return finalModules.map((row) => Array.from(row, (cell) => cell === 1))
}

function evaluatePenalty(grid: Uint8Array[], size: number): number {
  let penalty = 0
  for (let r = 0; r < size; r += 1) {
    let runColor = grid[r][0]
    let runLen = 1
    for (let c = 1; c < size; c += 1) {
      if (grid[r][c] === runColor) {
        runLen += 1
        if (runLen === 5) penalty += 3
        else if (runLen > 5) penalty += 1
      } else {
        runColor = grid[r][c]
        runLen = 1
      }
    }
  }
  for (let c = 0; c < size; c += 1) {
    let runColor = grid[0][c]
    let runLen = 1
    for (let r = 1; r < size; r += 1) {
      if (grid[r][c] === runColor) {
        runLen += 1
        if (runLen === 5) penalty += 3
        else if (runLen > 5) penalty += 1
      } else {
        runColor = grid[r][c]
        runLen = 1
      }
    }
  }
  return penalty
}

export function qrTextToDataUrl(text: string): string {
  const matrix = encodeQRCodeMatrix(text)
  if (!matrix) return ''
  const size = matrix.length
  const margin = 2
  const viewSize = size + margin * 2
  const rects: string[] = []
  for (let r = 0; r < size; r += 1) {
    for (let c = 0; c < size; c += 1) {
      if (matrix[r][c]) {
        rects.push(`<rect x="${c + margin}" y="${r + margin}" width="1" height="1"/>`)
      }
    }
  }
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${viewSize} ${viewSize}" shape-rendering="crispEdges"><rect width="${viewSize}" height="${viewSize}" fill="#ffffff"/><g fill="#111827">${rects.join('')}</g></svg>`
  return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`
}
