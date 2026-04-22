# temporal-quiz

Quiz generator + static UI for Temporal.io documentation. Previously lived in
two separate repos (`temporal-quiz-worker`, `temporal-quiz-ui`); now combined
here.

## Layout

```
.
├── worker/    Go worker. Scrapes temporalio/documentation, generates quizzes
│              via Claude, runs the full pipeline as a Temporal workflow.
└── ui/        Static HTML/CSS/JS quiz frontend. GitHub Pages serves ui/docs/.
```

## Common commands

```sh
cd worker
make pipeline     # scrape + generate + publish into ui/docs/quizzes
make worker       # run the Temporal worker locally
make test         # Go test suite
```

GitHub Pages source: this repo's `main` branch, `/ui/docs` folder.

See `worker/README.md` and `worker/Makefile` for the full worker command list.
