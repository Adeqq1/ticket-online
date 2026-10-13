<script lang="ts">
  import type { AdminApiEvent, StaffAssignment } from "../../lib/api.ts";

  let { events, value = $bindable(), pending = $bindable(false), disabled = false, mode = "edit" }: { events: AdminApiEvent[]; value: StaffAssignment[]; pending?: boolean; disabled?: boolean; mode?: "create" | "edit" } = $props();
  let eventId = $state("");
  let gate = $state("");
  let selectedEvent = $derived(events.find((event) => event.id === eventId));
  let gates = $derived([...new Set(selectedEvent?.ticketTiers.map((tier) => tier.gate.trim()).filter(Boolean) ?? [])]);
  let duplicate = $derived(value.some((item) => item.eventId === eventId && item.gate === gate));
  $effect(() => { pending = Boolean(eventId); });

  function changeEvent(event: Event) {
    eventId = (event.currentTarget as HTMLSelectElement).value;
    gate = "";
  }

  function addAssignment() {
    if (!eventId || !gates.includes(gate) || value.some((item) => item.eventId === eventId && item.gate === gate)) return;
    value = [...value, { eventId, gate }];
    eventId = ""; gate = "";
  }
</script>

<fieldset class="staff-assignment-editor" {disabled}>
  <legend class="visually-hidden">Penugasan event dan gate</legend>
  <p class="assignment-help">Pilih event → pilih gate → Tambah → {mode === "create" ? "Tinjau → Buat akun petugas" : "Simpan penugasan"}.</p>
  <p class="assignment-help">Tambah memasukkan pilihan ke draft. {mode === "create" ? "Penugasan disimpan bersama tombol Buat akun petugas." : "Akses baru berlaku setelah Simpan penugasan berhasil."}</p>
  {#if events.length}
    <div class="staff-assignment-controls">
      <label>Event
        <select value={eventId} onchange={changeEvent}>
          <option value="">Pilih event</option>
          {#each events as event (event.id)}<option value={event.id}>{event.artist} · {event.city}</option>{/each}
        </select>
      </label>
      <label>Gate
        <select bind:value={gate} disabled={!gates.length}>
          <option value="">Pilih gate</option>
          {#each gates as name (name)}<option value={name}>{name}</option>{/each}
        </select>
      </label>
      <button type="button" class="staff-secondary-button" disabled={!gate || !gates.includes(gate) || duplicate} onclick={addAssignment}>Tambah</button>
    </div>
    <div aria-live="polite" class="assignment-help">
      {#if eventId}
        <p>{!gates.length ? "Event ini belum memiliki gate. Pilih event lain atau batalkan pilihan." : duplicate ? "Penugasan event/gate ini sudah ada di draft. Pilih gate lain atau batalkan pilihan." : gate ? "Pilihan belum masuk draft. Tekan Tambah sebelum menyimpan, atau batalkan pilihan." : "Pilih gate, lalu tekan Tambah. Pilihan event ini belum masuk draft."}</p>
        <button type="button" class="staff-text-button" onclick={() => { eventId = ""; gate = ""; }}>Batalkan pilihan</button>
      {/if}
    </div>
  {:else}<p class="assignment-help">Event belum tersedia. {mode === "create" ? "Akun dapat disimpan tanpa penugasan; akses event/gate belum diberikan." : "Penugasan yang sudah tersimpan tetap ditampilkan."} Muat ulang katalog setelah event dan gate tersedia.</p>{/if}
  <ul class="staff-assignment-list">
    {#each value as assignment, index (`${assignment.eventId}:${assignment.gate}`)}
      <li><span>{events.find((item) => item.id === assignment.eventId)?.artist ?? assignment.eventId} · {assignment.gate}</span><button type="button" aria-label={`Hapus penugasan ${assignment.gate}`} {disabled} onclick={() => { value = value.filter((_, itemIndex) => itemIndex !== index); }}>Hapus</button></li>
    {/each}
  </ul>
</fieldset>

<style>
  .assignment-help { margin: 0 0 12px; color: var(--scan-muted); font-size: 14px; line-height: 1.6; }
  .assignment-help p { margin: 12px 0 4px; }
</style>
