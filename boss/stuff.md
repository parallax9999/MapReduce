potential problems:

When a worker program dies and comes back to life fast enough that the boss doesn't realize it's dead:
- The worker sends hello with the same worker id
- Boss overwrites the worker id in it's map to a new worker state
- Previous tasks get lost

Solution?:
- When the worker program dies and comes back to life, it will register itself with a new worker id.
- allows the boss to clean up the old worker state, requeue ActiveTasks
- Could still be an issue since we can have a bunch of "dead" workers in the map that will never come back to life. For example: it will always show as 2/3 or 3/4


Boss state struct is in high contention:
- Worker states are always being updated on each heartbeat (pong), Job map is always being updated for every TaskResult that a worker sends back (updates Task which updates Job). Also, the healthcheck loop updates the worker map.
- Temp fix by splitting the struct mutex into a mutex for the worker map and a mutex for Job map.
- But this can still be a bottleneck if we scale to many workers

Solution?:
- Use 2 channels dedicated for updates to the Worker and Job maps.
- 2 goroutines running in a loop popping from those channels and updating the maps
- When a routine want to update boss state, it simply submits to the channel.
- I think this fixes the lock contention but not sure?

Only 1 cpu core for each worker in this demo:
- It is possible for the boss to delagate >1 task per worker as long as the worker reports their max task limit.
- This decouples worker state and task state from each other, since a worker can be working on multiple tasks (and reporting TaskProgress for multiple tasks) at the same time.
- For this project, the workers are limited to 1 cpu core so they report that their max task limit is 1.