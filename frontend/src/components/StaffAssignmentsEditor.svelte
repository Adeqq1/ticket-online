<script lang="ts">
  import type { Concert } from "../lib/concerts.ts";
  import type { StaffAssignment } from "../lib/api.ts";

  let { events, value = $bindable(), disabled = false }: { events: Concert[]; value: StaffAssignment[]; disabled?: boolean } = $props();
  let eventId = $state("");
  let gate = $state("");
  let selectedEvent = $derived(events.find((event) => event.id === eventId));
  let gates = $derived([...new Set(selectedEvent?.ticketTiers.map((tier) => tier.gate.trim()).filter(Boolean) ?? [])]);

  function changeEvent(event: Event) {
    eventId = (event.currentTarget as HTMLSelectElement).value;
    gate = "";
  }

  function addAssignment() {
    if (!eventId || !gates.includes(gate) || value.some((item) => item.eventId === eventId && item.gate === gate)) return;
    value = [...value, { eventId, gate }];
  }
</script>

<fieldset class="staff-assignment-editor" {disabled}>
  <legend>Penugasan event dan gate</legend>
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
      <button type="button" class="staff-secondary-button" disabled={!gate || !gates.includes(gate) || value.some((item) => item.eventId === eventId && item.gate === gate)} onclick={addAssignment}>Tambah</button>
    </div>
  {:else}<p>Event belum tersedia. Muat ulang katalog sebelum membuat penugasan.</p>{/if}
  <ul class="staff-assignment-list">
    {#each value as assignment, index (`${assignment.eventId}:${assignment.gate}`)}
      <li><span>{events.find((item) => item.id === assignment.eventId)?.artist ?? assignment.eventId} · {assignment.gate}</span><button type="button" aria-label={`Hapus penugasan ${assignment.gate}`} {disabled} onclick={() => { value = value.filter((_, itemIndex) => itemIndex !== index); }}>Hapus</button></li>
    {/each}
  </ul>
</fieldset>
