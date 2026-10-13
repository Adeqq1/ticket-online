<script lang="ts">
  import { onDestroy, tick } from "svelte";
  import type { HTMLInputAttributes } from "svelte/elements";
  import { calendarDateLabel, calendarDays, calendarMonths, calendarWeekdays, dateFieldValue, daysInMonth, displayDate, isoDate, mondayIndex, parseDisplayDate, parseISODate, shiftCalendarDate, shiftCalendarMonth, type DateMode, type TimePrecision } from "../../lib/admin-date.ts";

  let { value = $bindable(""), pending = $bindable(false), id, label, showLabel = true, mode = "date", precision = "minute", timeZone, disabled = false, active = true, resetKey = 0, required = false, name, min, max, "aria-describedby": describedBy, "aria-invalid": ariaInvalid }: {
    value?: string; pending?: boolean; id: string; label: string; showLabel?: boolean; mode?: DateMode; precision?: TimePrecision; timeZone?: string; active?: boolean; resetKey?: number;
  } & Pick<HTMLInputAttributes, "disabled" | "required" | "name" | "min" | "max" | "aria-describedby" | "aria-invalid"> = $props();
  let field: HTMLInputElement;
  let trigger: HTMLButtonElement;
  let dialog: HTMLDialogElement;
  let date = $state("");
  let hour = $state("");
  let minute = $state("");
  let second = $state("");
  let published: string | undefined;
  let lastReset: number | undefined;
  let open = $state(false);
  let today = $state("");
  let focused = $state("2026-01-01");
  const viewed = $derived(parseISODate(focused)!);
  const days = $derived(calendarDays(viewed.year, viewed.month));
  let top = $state(0), left = $state(0), availableHeight = $state(600);

  $effect(() => {
    if (value !== published || resetKey !== lastReset) {
      published = value; lastReset = resetKey;
      const [d = "", t = ""] = value.split("T");
      date = displayDate(d);
      const parts = t.split(":");
      hour = parts[0] ?? ""; minute = parts[1] ?? ""; second = parts[2] ?? "";
      pending = false;
    }
  });
  $effect(() => {
    const result = dateFieldValue(date, hour, minute, second, mode, precision);
    const message = result === null ? "Isi tanggal DD/MM/YYYY dan waktu yang valid." : result && min && result < String(min) ? "Tanggal lebih awal dari batas yang diizinkan." : result && max && result > String(max) ? "Tanggal melewati batas yang diizinkan." : "";
    field?.setCustomValidity(message);
  });
  $effect(() => { if (disabled || !active) close(); });
  $effect(() => {
    if (!open) return;
    const observer = new MutationObserver(() => {
      if (field.matches(":disabled") || field.closest("[hidden]") || !field.getClientRects().length) close();
    });
    for (let element: HTMLElement | null = field; element; element = element.parentElement) observer.observe(element, { attributes: true, attributeFilter: ["disabled", "hidden", "class", "style"] });
    const viewport = window.visualViewport;
    window.addEventListener("resize", position);
    viewport?.addEventListener("resize", position);
    viewport?.addEventListener("scroll", position);
    return () => { observer.disconnect(); window.removeEventListener("resize", position); viewport?.removeEventListener("resize", position); viewport?.removeEventListener("scroll", position); };
  });
  onDestroy(() => dialog?.close());

  function update() {
    const result = dateFieldValue(date, hour, minute, second, mode, precision);
    pending = result === null;
    if (result !== null) { published = result; value = result; }
  }
  function typeDate(event: Event) {
    date = (event.currentTarget as HTMLInputElement).value;
    if (!date) hour = minute = second = "";
    else if (parseDisplayDate(date) && mode === "datetime" && !hour && !minute && !second) { hour = minute = "00"; second = precision === "second" ? "00" : ""; }
    update();
  }
  function typeTime(event: Event, part: "hour" | "minute" | "second") {
    const text = (event.currentTarget as HTMLInputElement).value;
    if (part === "hour") hour = text;
    else if (part === "minute") minute = text;
    else second = text;
    update();
  }
  async function position() {
    if (!dialog?.open) return;
    const viewport = window.visualViewport;
    const width = viewport?.width ?? innerWidth, height = viewport?.height ?? innerHeight;
    const x = viewport?.offsetLeft ?? 0, y = viewport?.offsetTop ?? 0;
    availableHeight = height - 8;
    await tick();
    if (!dialog.open) return;
    const bounds = dialog.getBoundingClientRect(), anchor = field.getBoundingClientRect();
    left = x + Math.max(2, Math.min(width - bounds.width - 2, innerWidth < 768 ? (width - bounds.width) / 2 : anchor.left - x));
    top = y + Math.max(4, Math.min(height - Math.min(bounds.height, availableHeight) - 4, innerWidth < 768 ? (height - Math.min(bounds.height, availableHeight)) / 2 : anchor.bottom + 8 + bounds.height <= y + height ? anchor.bottom + 8 - y : anchor.top - bounds.height - 8 - y));
  }
  async function show() {
    if (disabled || !active || field.matches(":disabled")) return;
    const parts = new Intl.DateTimeFormat("en-GB", { timeZone, year: "numeric", month: "numeric", day: "numeric" }).formatToParts(new Date());
    const part = (type: Intl.DateTimeFormatPartTypes) => Number(parts.find(item => item.type === type)!.value);
    today = isoDate(part("year"), part("month"), part("day"));
    focused = parseDisplayDate(date) ?? today;
    field.blur();
    dialog.showModal(); open = true;
    await tick(); await position();
    await focusDay();
  }
  function close() {
    if (!dialog?.open) return;
    dialog.close(); open = false;
    if (!trigger.matches(":disabled") && trigger.getClientRects().length) trigger.focus({ preventScroll: true });
  }
  async function choose(day: string) {
    date = displayDate(day);
    if (mode === "datetime") {
      hour ||= "00"; minute ||= "00";
      if (precision === "second") second ||= "00";
    }
    update();
    await tick();
    field.dispatchEvent(new Event("input", { bubbles: true }));
    close();
  }
  async function clear() {
    date = hour = minute = second = ""; update();
    await tick(); field.dispatchEvent(new Event("input", { bubbles: true })); close();
  }
  async function focusDay() {
    await tick(); dialog.querySelector<HTMLButtonElement>(`[data-date="${focused}"]`)?.focus();
  }
  function changeView(year: number, month: number) {
    focused = isoDate(year, month, Math.min(viewed.day, daysInMonth(year, month)));
  }
  function dayKey(event: KeyboardEvent) {
    const day = (event.target as HTMLElement).dataset.date;
    if (!day) return;
    let next: string;
    if (event.key === "ArrowLeft") next = shiftCalendarDate(day, -1);
    else if (event.key === "ArrowRight") next = shiftCalendarDate(day, 1);
    else if (event.key === "ArrowUp") next = shiftCalendarDate(day, -7);
    else if (event.key === "ArrowDown") next = shiftCalendarDate(day, 7);
    else if (event.key === "Home") next = shiftCalendarDate(day, -mondayIndex(day));
    else if (event.key === "End") next = shiftCalendarDate(day, 6 - mondayIndex(day));
    else if (event.key === "PageUp") next = shiftCalendarMonth(day, event.shiftKey ? -12 : -1);
    else if (event.key === "PageDown") next = shiftCalendarMonth(day, event.shiftKey ? 12 : 1);
    else return;
    event.preventDefault(); focused = next; void focusDay();
  }
</script>

<div class="admin-date-field">
  {#if showLabel}<label for={id}>{label}</label>{/if}
  <div class="date-entry">
    <input bind:this={field} {id} {name} type="text" inputmode="text" autocomplete="off" placeholder="DD/MM/YYYY" maxlength="10" value={date} oninput={typeDate} {required} {disabled} aria-describedby={describedBy} aria-invalid={ariaInvalid ?? (pending ? "true" : undefined)} />
    <button bind:this={trigger} class="date-trigger" type="button" {disabled} aria-label={`Buka kalender ${label}`} aria-haspopup="dialog" aria-expanded={open} aria-controls={`${id}-calendar`} onclick={show}><svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" aria-hidden="true"><path d="M5 5h14v15H5zM8 3v4m8-4v4M5 10h14" /></svg></button>
  </div>
  {#if mode === "datetime"}
    <div class="date-time">
      {#each ["hour", "minute", ...(precision === "second" ? ["second"] : [])] as part}
        {@const title = part === "hour" ? "Jam" : part === "minute" ? "Menit" : "Detik"}
        <label>{title}<input type="number" inputmode="numeric" min="0" max={part === "hour" ? 23 : 59} step="1" value={part === "hour" ? hour : part === "minute" ? minute : second} oninput={event => typeTime(event, part as "hour" | "minute" | "second")} required={Boolean(date)} {disabled} aria-label={`${title} ${label}`} aria-describedby={describedBy} /></label>
      {/each}
    </div>
  {/if}
  <dialog bind:this={dialog} id={`${id}-calendar`} class="admin-calendar" aria-label={`Kalender ${label}`} style:top={`${top}px`} style:left={`${left}px`} style:max-height={`${availableHeight}px`} oncancel={event => { event.preventDefault(); close(); }} onclick={event => { if (event.target === dialog) { const r = dialog.getBoundingClientRect(); if (event.clientX < r.left || event.clientX > r.right || event.clientY < r.top || event.clientY > r.bottom) close(); } }} onclose={() => open = false}>
    <div class="calendar-heading"><strong aria-live="polite">{calendarMonths[viewed.month - 1]} {viewed.year}</strong><button type="button" disabled={!open} aria-label="Bulan sebelumnya" onclick={() => focused = shiftCalendarMonth(focused, -1)}>‹</button><button type="button" disabled={!open} aria-label="Bulan berikutnya" onclick={() => focused = shiftCalendarMonth(focused, 1)}>›</button></div>
    <div class="calendar-navigation">
      <label for={`${id}-month`}>Bulan<select id={`${id}-month`} disabled={!open} value={viewed.month} onchange={event => changeView(viewed.year, Number(event.currentTarget.value))}>{#each calendarMonths as month, i}<option value={i + 1}>{month}</option>{/each}</select></label>
      <label for={`${id}-year`}>Tahun<input id={`${id}-year`} type="number" inputmode="numeric" min="1" max="9999" step="1" disabled={!open} value={viewed.year} onchange={event => { const year = Number(event.currentTarget.value); if (Number.isInteger(year) && year >= 1 && year <= 9999) changeView(year, viewed.month); }} onkeydown={event => { if (event.key === "Enter") { event.preventDefault(); event.currentTarget.blur(); } }} onblur={event => event.currentTarget.value = String(viewed.year)} /></label>
    </div>
    <table role="grid" aria-label={`${calendarMonths[viewed.month - 1]} ${viewed.year}`} onkeydown={dayKey}>
      <thead><tr>{#each calendarWeekdays as day}<th scope="col" aria-label={day}>{day.slice(0, 3)}</th>{/each}</tr></thead>
      <tbody>{#each Array.from({ length: 6 }, (_, i) => i) as week}<tr>{#each days.slice(week * 7, week * 7 + 7) as day}<td aria-selected={Boolean(day && day === parseDisplayDate(date))}>{#if day}<button type="button" data-date={day} disabled={!open} tabindex={day === focused ? 0 : -1} class:outside={parseISODate(day)!.month !== viewed.month} class:selected={day === parseDisplayDate(date)} aria-current={day === today ? "date" : undefined} aria-label={calendarDateLabel(day)} onclick={() => choose(day)}>{parseISODate(day)!.day}</button>{/if}</td>{/each}</tr>{/each}</tbody>
    </table>
    <div class="calendar-actions"><button type="button" disabled={!open} onclick={() => choose(today)}>Hari ini</button><button type="button" disabled={!open} onclick={clear}>Kosongkan</button><button type="button" disabled={!open} onclick={close}>Tutup</button></div>
  </dialog>
</div>

<style>
  .admin-date-field { display: grid; gap: 6px; min-width: 0; align-content: start; }
  .admin-date-field label { display: grid; min-width: 0; gap: 6px; }
  .date-entry { display: grid; grid-template-columns: minmax(0, 1fr) 44px; gap: 6px; }
  .admin-date-field input, .admin-date-field select { box-sizing: border-box; width: 100%; min-width: 0; min-height: 44px; padding: 8px 10px; }
  .date-trigger { display: grid; place-items: center; padding: 0; }
  .date-time { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; }
  .date-time:has(label:nth-child(3)) { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  .admin-calendar { position: fixed; box-sizing: border-box; margin: 0; width: min(24rem, calc(100vw - 4px)); max-width: none; padding: 12px; overflow: auto; overscroll-behavior: contain; border: 1px solid var(--admin-control-border); border-radius: 16px; background: var(--admin-card); color: var(--admin-text); font: inherit; box-shadow: 0 16px 48px #0008; }
  .admin-calendar::backdrop { background: #07110c80; }
  .calendar-heading { display: grid; grid-template-columns: minmax(0, 1fr) 44px 44px; gap: 6px; align-items: center; }
  .calendar-navigation { display: grid; grid-template-columns: minmax(0, 1.3fr) minmax(0, 1fr); gap: 8px; margin-block: 12px; }
  .admin-date-field button { min-width: 44px; min-height: 44px; border: 1px solid var(--admin-control-border); border-radius: 10px; background: var(--admin-input); color: var(--admin-text); font: inherit; cursor: pointer; }
  .admin-date-field button:hover:not(:disabled) { background: var(--admin-neutral-bg); }
  .admin-calendar table { width: 100%; table-layout: fixed; border-collapse: collapse; font: inherit; }
  .admin-calendar th { padding: 4px 0; font-weight: 400; font-size: .875em; color: var(--admin-secondary); text-align: center; }
  .admin-calendar td { padding: 0; border: 0; text-align: center; }
  .admin-calendar td button { display: block; width: 100%; padding: 0; border-color: transparent; }
  .admin-calendar .outside { color: var(--admin-secondary); }
  .admin-calendar button[aria-current="date"] { border-color: var(--admin-accent); }
  .admin-calendar button.selected { background: var(--admin-accent); color: var(--admin-canvas); }
  .calendar-actions { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 12px; }
  .calendar-actions button { flex: 1 1 auto; padding: 6px; }
  @media (max-width: 360px) { .admin-calendar { padding: 3px; } }
  @media print { .admin-calendar, .date-trigger { display: none; } }
</style>
