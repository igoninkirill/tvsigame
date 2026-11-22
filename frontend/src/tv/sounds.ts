// Озвучка ТВ-экрана «Своей Игры».
//
// Аутентичные сэмплы телеигры — из открытого проекта SIGame
// (https://github.com/VladimirKhil/SI, лицензия MIT, (c) VladimirKhil).
// Файлы лежат в frontend/public/sounds/*.mp3 и отдаются как статика.
//
// Браузеры разрешают звук только после жеста пользователя: первый тап/клик
// по экрану ТВ разблокирует последующее воспроизведение (до этого play()
// молча отклоняется — игра просто идёт без звука).

let enabled = true

try {
  enabled = localStorage.getItem('sigame:sound') !== 'off'
} catch {
  /* приватный режим */
}

export function soundEnabled(): boolean {
  return enabled
}

export function setSoundEnabled(v: boolean) {
  enabled = v
  try {
    localStorage.setItem('sigame:sound', v ? 'on' : 'off')
  } catch {
    /* некритично */
  }
  if (!v) stopAll()
}

const BASE = '/sounds/'
const playing = new Map<string, HTMLAudioElement>()

function play(name: string, volume = 1): HTMLAudioElement | null {
  if (!enabled) return null
  stopSound(name)
  const a = new Audio(`${BASE}${name}.mp3`)
  a.volume = volume
  playing.set(name, a)
  a.addEventListener('ended', () => {
    if (playing.get(name) === a) playing.delete(name)
  })
  void a.play().catch(() => {
    // Автоплей заблокирован (не было жеста) — пропускаем без шума в консоль.
    if (playing.get(name) === a) playing.delete(name)
  })
  return a
}

export function stopSound(name: string) {
  const a = playing.get(name)
  if (a) {
    a.pause()
    playing.delete(name)
  }
}

function stopAll() {
  for (const name of [...playing.keys()]) stopSound(name)
}

// Предзагрузка после первого жеста, чтобы первый сэмпл не заикался.
if (typeof document !== 'undefined') {
  const warm = () => {
    for (const n of ['round_begin', 'round_themes']) {
      const a = new Audio(`${BASE}${n}.mp3`)
      a.preload = 'auto'
      void a.load()
    }
  }
  document.addEventListener('pointerdown', warm, { capture: true, passive: true, once: true })
}

let introActive = false

/** Заставка раунда: джингл начала раунда, затем музыка объявления тем. */
export function playRoundIntro() {
  introActive = true
  const jingle = play('round_begin')
  if (jingle) {
    jingle.addEventListener('ended', () => {
      // Музыка тем — только если заставку не пропустили.
      if (introActive) play('round_themes', 0.85)
    })
  } else {
    play('round_themes', 0.85)
  }
}

/** Остановить музыку заставки (ведущий нажал «К табло» / раунд начался). */
export function stopRoundIntro() {
  introActive = false
  stopSound('round_begin')
  stopSound('round_themes')
}

/** Кот в мешке. */
export function playCat() {
  play('question_secret')
}

/** Аукцион. */
export function playAuction() {
  play('question_stake')
}

/** Вопрос для всех. */
export function playForAll() {
  play('question_all')
}

/** Вопрос без риска. */
export function playNoRisk() {
  play('question_norisk')
}

/** Время вышло, никто не ответил. */
export function playNoAnswers() {
  play('question_noanswers')
}

/** Верный ответ: небольшие аплодисменты. */
export function playCorrect() {
  play('applause_small', 0.8)
}

/** Неверный ответ: фирменный «бззз». */
export function playWrong() {
  play('answer_wrong')
}

/** Исключение темы финала. */
export function playFinalDelete() {
  play('final_delete')
}

/** Финальный вопрос: легендарные 60 секунд на размышление. */
export function playFinalQuestion() {
  play('final_think', 0.9)
}

export function stopFinalQuestion() {
  stopSound('final_think')
}

/** Итоги игры: финальные аплодисменты. */
export function playResults() {
  play('applause_final')
}
