# Sangeeth's WhatsApp Automations

Currently has the following main functionalities:

- **Waxit:** Annoy my contacts to switch over to Signal whenever they DM
- **Solar:** Collect solar power generation metrics and send a daily

## TODOs

- [ ] Better project organization since this evolved beyond the signal thingy
- [ ] Add systemd unit file to repo, update on deploys (if needed)
- [ ] Remove hardcoded internal hostnames
- [ ] Solar
    - [ ] Persist the collected metrics, load during init (incase of powercuts, could help)
    - [ ] Comparison with yesterday
    - [ ] Overkill: Collect periodic metrics in a slice, persist
    - [ ] Overkill: Weather info reporting, maybe using this to create better summaries
    - [ ] Overkill: Send daily generation graph
