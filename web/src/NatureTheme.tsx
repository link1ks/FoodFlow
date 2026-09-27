import {Leaf, Sprout, Utensils} from 'lucide-react'

/** Decorative artwork stays outside the interaction and accessibility trees. */
export function NatureBackdrop() {
  return <div className="nature-backdrop" aria-hidden="true">
    <svg className="nature-branch" viewBox="0 0 260 460" fill="none">
      <path d="M240 440C90 325 180 150 40 10" stroke="currentColor" strokeWidth="2"/>
      <path d="M140 330C33 327 28 262 35 214C99 216 153 255 140 330ZM133 254C224 237 247 174 223 124C164 145 125 190 133 254ZM112 159C41 166 5 102 13 61C72 72 111 105 112 159ZM76 77C147 61 158 30 151 4C100 13 75 36 76 77Z" fill="currentColor" fillOpacity=".3"/>
    </svg>
    <div className="nature-orbit"/>
  </div>
}

export function NatureWelcome() {
  return <section className="nature-welcome" aria-label="食光生活理念">
    <div className="relative z-10">
      <p className="mb-2 flex items-center gap-2 text-xs font-semibold tracking-widest text-emerald-800"><Sprout size={16} aria-hidden="true"/> 食光 · 自然有序的每一天</p>
      <h2 className="text-2xl font-bold leading-snug text-emerald-950 sm:text-3xl">好好吃饭，少一点浪费。</h2>
      <p className="mt-3 max-w-md text-sm leading-6 text-emerald-900/80">从冰箱里的一份新鲜，到餐桌上的一顿用心。<br className="hidden sm:block"/>让每一份食材，都恰好用在喜欢的生活里。</p>
      <div className="mt-4 flex flex-wrap gap-3 text-xs font-medium text-emerald-800">
        <span className="nature-pill"><Leaf size={14} aria-hidden="true"/> 珍惜食材</span>
        <span className="nature-pill"><Utensils size={14} aria-hidden="true"/> 按需备餐</span>
      </div>
    </div>
    <svg className="nature-harvest" viewBox="0 0 320 220" fill="none" aria-hidden="true">
      <circle cx="170" cy="112" r="97" fill="#e1edd5"/>
      <ellipse cx="171" cy="191" rx="111" ry="12" fill="#29694c" fillOpacity=".09"/>
      <path d="M158 151C112 126 103 79 126 37C170 54 188 104 158 151Z" fill="#81a86c"/>
      <path d="M160 155C151 105 187 62 232 65C229 115 198 150 160 155Z" fill="#a8c88d"/>
      <path d="M160 153L130 59M162 148L215 80" stroke="#f5f8e8" strokeWidth="2" strokeLinecap="round"/>
      <circle cx="119" cy="141" r="32" fill="#df8a65"/>
      <path d="M119 110L113 100M119 110L105 114M119 110L131 105" stroke="#48724a" strokeWidth="5" strokeLinecap="round"/>
      <ellipse cx="211" cy="145" rx="26" ry="33" transform="rotate(25 211 145)" fill="#efd8a1"/>
      <path d="M72 152H264L240 187H96L72 152Z" fill="#f8faf1" stroke="#668963" strokeWidth="2" strokeLinejoin="round"/>
      <path d="M105 165H230" stroke="#c3d3b7" strokeWidth="2" strokeLinecap="round"/>
      <circle cx="69" cy="73" r="4" fill="#c3a968"/><circle cx="268" cy="118" r="3" fill="#81a86c"/>
    </svg>
  </section>
}
